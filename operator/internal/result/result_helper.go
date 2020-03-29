// Copyright DataStax, Inc.
// Please see the included license file for details.

package result

import (
	"time"

	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type ReconcileResult interface {
	Completed() bool
	Output() (reconcile.Result, error)
}

type continueReconcile struct{}

func (c continueReconcile) Completed() bool {
	return false
}
func (c continueReconcile) Output() (reconcile.Result, error) {
	panic("there was no Result to return")
}

type done struct{}

func (d done) Completed() bool {
	return true
}
func (d done) Output() (reconcile.Result, error) {
	return reconcile.Result{}, nil
}

type callBackSoon struct {
	secs int
}

func (c callBackSoon) Completed() bool {
	return true
}
func (c callBackSoon) Output() (reconcile.Result, error) {
	t := time.Duration(c.secs) * time.Second
	return reconcile.Result{Requeue: true, RequeueAfter: t}, nil
}

type errorOut struct {
	err error
}

func (e errorOut) Completed() bool {
	return true
}
func (e errorOut) Output() (reconcile.Result, error) {
	return reconcile.Result{}, e.err
}

func Continue() ReconcileResult {
	return continueReconcile{}
}

func Done() ReconcileResult {
	return done{}
}

func RequeueSoon(secs int) ReconcileResult {
	return callBackSoon{secs: secs}
}

func Error(e error) ReconcileResult {
	return errorOut{err: e}
}
