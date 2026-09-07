package main

import (
	"context"
	"time"

	"agentcommons/internal/core"
)

type attachmentLease struct {
	client     rpcClient
	attachment core.Attachment
}

func (connection projectConnection) holdAttachment(ctx context.Context, attachment core.Attachment,
	options onboardingOptions, streams commandStreams) error {
	child, cancel := context.WithCancel(ctx)
	defer cancel()
	lease := attachmentLease{client: connection.client, attachment: attachment}
	renewed := lease.startRenewal(child, cancel)
	watchErr := runWatch(child, connection.watchArguments(options), streams.Output, streams.Errors)
	cancel()
	renewErr := <-renewed
	lease.release()
	if renewErr != nil {
		return renewErr
	}
	return watchErr
}

func (connection projectConnection) watchArguments(options onboardingOptions) []string {
	config := connection.config
	args := []string{"--state", config.State, "--socket", config.Socket, "--token-file", config.TokenFile}
	if options.Runtime == "codex" {
		args = append(args, "--codex-thread", options.NativeSession)
	}
	if options.Once {
		args = append(args, "--once")
	}
	return args
}

func (lease attachmentLease) startRenewal(ctx context.Context, cancel context.CancelFunc) <-chan error {
	done := make(chan error, 1)
	go func() { err := lease.renewUntilCanceled(ctx); done <- err; cancel() }()
	return done
}

func (lease attachmentLease) renewUntilCanceled(ctx context.Context) error {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	return lease.renewOnTicks(ctx, ticker.C)
}

func (lease attachmentLease) renewOnTicks(ctx context.Context, ticks <-chan time.Time) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticks:
			if _, err := lease.call(ctx, "sessions.renew"); err != nil {
				return err
			}
		}
	}
}

func (lease attachmentLease) call(ctx context.Context, method string) (core.Attachment, error) {
	return rpcCall[core.Attachment](ctx, lease.client, method, map[string]string{
		"nativeId": lease.attachment.NativeID, "leaseId": lease.attachment.LeaseID,
	})
}

func (lease attachmentLease) release() {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, _ = lease.call(ctx, "sessions.detach")
}
