package temporalops

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

const (
	OperationWorkflowName      = "safeclaw.nondet.operation.workflow"
	OperationActivityName      = "safeclaw.nondet.operation.activity"
	OperationBatchWorkflowName = "safeclaw.nondet.operation.batch.workflow"
	OperationBatchActivityName = "safeclaw.nondet.operation.batch.activity"
)

type Executor interface {
	Execute(ctx context.Context, op string, payload []byte) ([]byte, error)
	ExecuteWithWorkflowContext(ctx workflow.Context, op string, payload []byte) ([]byte, error)
	ExecuteBatch(ctx context.Context, reqs []OperationRequest) ([][]byte, error)
	Close() error
}

type SDKConfig struct {
	HostPort  string
	Namespace string
	TaskQueue string
}

type LocalExecutor struct{}

func (e *LocalExecutor) Execute(ctx context.Context, op string, payload []byte) ([]byte, error) {
	return ExecuteLocal(ctx, op, payload)
}
func (e *LocalExecutor) ExecuteWithWorkflowContext(ctx workflow.Context, op string, payload []byte) ([]byte, error) {
	return ExecuteLocal(context.Background(), op, payload)
}
func (e *LocalExecutor) ExecuteBatch(ctx context.Context, reqs []OperationRequest) ([][]byte, error) {
	out := make([][]byte, 0, len(reqs))
	for _, req := range reqs {
		raw, err := ExecuteLocalWithPolicy(ctx, req.Op, req.Payload, req.ScriptPolicy)
		if err != nil {
			return nil, err
		}
		out = append(out, raw)
	}
	return out, nil
}

func (e *LocalExecutor) Close() error { return nil }

type SDKExecutor struct {
	cfg          SDKConfig
	scriptPolicy ScriptPolicy
	client       client.Client
	worker       worker.Worker
	once         sync.Once
	err          error
}

func NewSDKExecutor(cfg SDKConfig, scriptPolicy ScriptPolicy) *SDKExecutor {
	return &SDKExecutor{cfg: cfg, scriptPolicy: scriptPolicy}
}

func (e *SDKExecutor) Execute(ctx context.Context, op string, payload []byte) ([]byte, error) {
	if err := e.ensureStarted(); err != nil {
		return nil, err
	}
	req := OperationRequest{Op: op, Payload: payload, ScriptPolicy: e.scriptPolicy}
	run, err := e.client.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID:        fmt.Sprintf("safeclaw-nondet-%d", time.Now().UnixNano()),
		TaskQueue: e.cfg.TaskQueue,
	}, OperationWorkflowName, req)
	if err != nil {
		return nil, err
	}
	var out []byte
	if err := run.Get(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (e *SDKExecutor) Close() error {
	if e.worker != nil {
		e.worker.Stop()
	}
	if e.client != nil {
		e.client.Close()
	}
	return nil
}

func (e *SDKExecutor) ExecuteWithWorkflowContext(ctx workflow.Context, op string, payload []byte) ([]byte, error) {
	req := OperationRequest{Op: op, Payload: payload, ScriptPolicy: e.scriptPolicy}
	ao := workflow.ActivityOptions{StartToCloseTimeout: 30 * time.Second}
	ctx = workflow.WithActivityOptions(ctx, ao)
	var out []byte
	if err := workflow.ExecuteActivity(ctx, OperationActivityName, req).Get(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (e *SDKExecutor) ExecuteBatch(ctx context.Context, reqs []OperationRequest) ([][]byte, error) {
	if err := e.ensureStarted(); err != nil {
		return nil, err
	}
	for i := range reqs {
		if len(reqs[i].ScriptPolicy.Allowlist) == 0 {
			reqs[i].ScriptPolicy = e.scriptPolicy
		}
	}
	run, err := e.client.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID:        fmt.Sprintf("safeclaw-nondet-batch-%d", time.Now().UnixNano()),
		TaskQueue: e.cfg.TaskQueue,
	}, OperationBatchWorkflowName, reqs)
	if err != nil {
		return nil, err
	}
	var out [][]byte
	if err := run.Get(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (e *SDKExecutor) ensureStarted() error {
	e.once.Do(func() {
		if e.cfg.HostPort == "" {
			e.cfg.HostPort = "localhost:7233"
		}
		if e.cfg.Namespace == "" {
			e.cfg.Namespace = "default"
		}
		if e.cfg.TaskQueue == "" {
			e.cfg.TaskQueue = "safeclaw-nondet"
		}
		c, err := client.Dial(client.Options{
			HostPort:  e.cfg.HostPort,
			Namespace: e.cfg.Namespace,
		})
		if err != nil {
			e.err = err
			return
		}
		w := worker.New(c, e.cfg.TaskQueue, worker.Options{})
		w.RegisterWorkflowWithOptions(OperationWorkflow, workflow.RegisterOptions{Name: OperationWorkflowName})
		w.RegisterWorkflowWithOptions(OperationBatchWorkflow, workflow.RegisterOptions{Name: OperationBatchWorkflowName})
		w.RegisterActivityWithOptions(OperationActivity, activity.RegisterOptions{Name: OperationActivityName})
		w.RegisterActivityWithOptions(OperationBatchActivity, activity.RegisterOptions{Name: OperationBatchActivityName})
		if err := w.Start(); err != nil {
			c.Close()
			e.err = err
			return
		}
		e.client = c
		e.worker = w
	})
	return e.err
}

type TestsuiteExecutor struct {
	suite        testsuite.WorkflowTestSuite
	scriptPolicy ScriptPolicy
}

func NewTestsuiteExecutor(scriptPolicy ScriptPolicy) *TestsuiteExecutor {
	return &TestsuiteExecutor{scriptPolicy: scriptPolicy}
}

func (e *TestsuiteExecutor) Execute(ctx context.Context, op string, payload []byte) ([]byte, error) {
	env := e.suite.NewTestWorkflowEnvironment()
	env.RegisterWorkflowWithOptions(OperationWorkflow, workflow.RegisterOptions{Name: OperationWorkflowName})
	env.RegisterActivityWithOptions(OperationActivity, activity.RegisterOptions{Name: OperationActivityName})
	req := OperationRequest{Op: op, Payload: payload, ScriptPolicy: e.scriptPolicy}
	env.ExecuteWorkflow(OperationWorkflow, req)
	if err := env.GetWorkflowError(); err != nil {
		return nil, err
	}
	var out []byte
	if err := env.GetWorkflowResult(&out); err != nil {
		return nil, err
	}
	return out, nil
}

func (e *TestsuiteExecutor) Close() error { return nil }

func (e *TestsuiteExecutor) ExecuteWithWorkflowContext(ctx workflow.Context, op string, payload []byte) ([]byte, error) {
	req := OperationRequest{Op: op, Payload: payload, ScriptPolicy: e.scriptPolicy}
	var out []byte
	if err := workflow.ExecuteActivity(ctx, OperationActivityName, req).Get(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (e *TestsuiteExecutor) ExecuteBatch(ctx context.Context, reqs []OperationRequest) ([][]byte, error) {
	env := e.suite.NewTestWorkflowEnvironment()
	env.RegisterWorkflowWithOptions(OperationBatchWorkflow, workflow.RegisterOptions{Name: OperationBatchWorkflowName})
	env.RegisterActivityWithOptions(OperationBatchActivity, activity.RegisterOptions{Name: OperationBatchActivityName})
	for i := range reqs {
		if len(reqs[i].ScriptPolicy.Allowlist) == 0 {
			reqs[i].ScriptPolicy = e.scriptPolicy
		}
	}
	env.ExecuteWorkflow(OperationBatchWorkflow, reqs)
	if err := env.GetWorkflowError(); err != nil {
		return nil, err
	}
	var out [][]byte
	if err := env.GetWorkflowResult(&out); err != nil {
		return nil, err
	}
	return out, nil
}

func OperationWorkflow(ctx workflow.Context, req OperationRequest) ([]byte, error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)
	var out []byte
	err := workflow.ExecuteActivity(ctx, OperationActivityName, req).Get(ctx, &out)
	return out, err
}

func OperationActivity(ctx context.Context, req OperationRequest) ([]byte, error) {
	return ExecuteLocalWithPolicy(ctx, req.Op, req.Payload, req.ScriptPolicy)
}

func OperationBatchWorkflow(ctx workflow.Context, reqs []OperationRequest) ([][]byte, error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)
	var out [][]byte
	err := workflow.ExecuteActivity(ctx, OperationBatchActivityName, reqs).Get(ctx, &out)
	return out, err
}

func OperationBatchActivity(ctx context.Context, reqs []OperationRequest) ([][]byte, error) {
	out := make([][]byte, 0, len(reqs))
	for _, req := range reqs {
		raw, err := ExecuteLocalWithPolicy(ctx, req.Op, req.Payload, req.ScriptPolicy)
		if err != nil {
			return nil, err
		}
		out = append(out, raw)
	}
	return out, nil
}
