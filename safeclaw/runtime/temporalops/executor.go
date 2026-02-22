package temporalops

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

const (
	OperationWorkflowName = "safeclaw.nondet.operation.workflow"
	OperationActivityName = "safeclaw.nondet.operation.activity"
)

type Executor interface {
	Execute(ctx context.Context, op string, payload []byte) ([]byte, error)
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

func (e *LocalExecutor) Close() error { return nil }

type SDKExecutor struct {
	cfg    SDKConfig
	client client.Client
	worker worker.Worker
	once   sync.Once
	err    error
}

func NewSDKExecutor(cfg SDKConfig) *SDKExecutor {
	return &SDKExecutor{cfg: cfg}
}

func (e *SDKExecutor) Execute(ctx context.Context, op string, payload []byte) ([]byte, error) {
	if err := e.ensureStarted(); err != nil {
		return nil, err
	}
	req := OperationRequest{Op: op, Payload: payload}
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
		w.RegisterActivityWithOptions(OperationActivity, activity.RegisterOptions{Name: OperationActivityName})
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
	suite testsuite.WorkflowTestSuite
}

func NewTestsuiteExecutor() *TestsuiteExecutor {
	return &TestsuiteExecutor{}
}

func (e *TestsuiteExecutor) Execute(ctx context.Context, op string, payload []byte) ([]byte, error) {
	env := e.suite.NewTestWorkflowEnvironment()
	env.RegisterWorkflowWithOptions(OperationWorkflow, workflow.RegisterOptions{Name: OperationWorkflowName})
	env.RegisterActivityWithOptions(OperationActivity, activity.RegisterOptions{Name: OperationActivityName})
	req := OperationRequest{Op: op, Payload: payload}
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
	return ExecuteLocal(ctx, req.Op, req.Payload)
}
