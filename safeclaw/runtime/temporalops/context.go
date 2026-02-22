package temporalops

import (
	"context"

	"go.temporal.io/sdk/workflow"
)

type workflowContextKey string

const WorkflowContextKey workflowContextKey = "safeclaw_temporal_workflow_context"

func WithWorkflowContext(ctx context.Context, wfCtx workflow.Context) context.Context {
	return context.WithValue(ctx, WorkflowContextKey, wfCtx)
}

func GetWorkflowContext(ctx context.Context) (workflow.Context, bool) {
	v := ctx.Value(WorkflowContextKey)
	wfCtx, ok := v.(workflow.Context)
	return wfCtx, ok
}
