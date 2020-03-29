// Copyright DataStax, Inc.
// Please see the included license file for details.

package result

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestContinue(t *testing.T) {
	t.Run("test Continue() fitting the interface", func(t *testing.T) {
		c := Continue()
		assert.False(t, c.Completed(), "Continue() output should say reconciling is not complete")

		assert.Panics(
			t,
			func() {
				res, err := c.Output()
				fmt.Println(res)
				fmt.Println(err)
			}, "Output() should panic for Continue()")
	})
}

func TestDone(t *testing.T) {
	t.Run("test Done() fitting the interface", func(t *testing.T) {
		d := Done()
		assert.True(t, d.Completed(), "Done() output should say reconciling is complete")

		res, err := d.Output()
		assert.Equal(t, reconcile.Result{}, res, "Done() should return the zero value reconcile.Result{}")

		assert.NoError(t, err, "Done() should not have an error")
	})
}

func TestError(t *testing.T) {
	t.Run("test Continue() fitting the interface", func(t *testing.T) {
		e := Error(fmt.Errorf("problem message"))

		assert.True(t, e.Completed(), "Error() output should say reconciling is complete")

		_, err := e.Output()

		assert.Error(t, err, "Error() should have an error")
	})
}

func TestRequeueSoon(t *testing.T) {
	t.Run("test RequeueSoon() fitting the interface", func(t *testing.T) {
		r := RequeueSoon(10)

		assert.True(t, r.Completed(), "RequeueSoon() output should say reconciling is complete")

		res, err := r.Output()

		assert.NoError(t, err, "RequeueSoon() should not have an error")

		tenSecs := time.Second * 10
		assert.Equal(t, reconcile.Result{Requeue: true, RequeueAfter: tenSecs}, res,
			"RequeueSoon() should return a reconcile.Result{} with Requeue=true and 10 seconds")
	})
}
