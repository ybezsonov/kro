package finalizer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHaveAtLeastOneFinalizer(t *testing.T) {
	manager := &Manager{
		finalizers: []string{"finalizer1", "finalizer2"},
	}

	t.Run("at least one finalizer", func(t *testing.T) {
		strs := []string{"finalizer1", "finalizer3"}
		result := manager.HaveAtLeastOneFinalizer(strs)
		assert.True(t, result)
	})

	t.Run("no finalizers", func(t *testing.T) {
		strs := []string{"finalizer3", "finalizer4"}
		result := manager.HaveAtLeastOneFinalizer(strs)
		assert.False(t, result)
	})
}

func TestHaveAllFinalizers(t *testing.T) {
	manager := &Manager{
		finalizers: []string{"finalizer1", "finalizer2"},
	}

	t.Run("all finalizers", func(t *testing.T) {
		strs := []string{"finalizer1", "finalizer2"}
		result := manager.HaveAllFinalizers(strs)
		assert.True(t, result)
	})

	t.Run("not all finalizers", func(t *testing.T) {
		strs := []string{"finalizer1", "finalizer3"}
		result := manager.HaveAllFinalizers(strs)
		assert.False(t, result)
	})

	t.Run("no finalizers", func(t *testing.T) {
		strs := []string{"finalizer3", "finalizer4"}
		result := manager.HaveAllFinalizers(strs)
		assert.False(t, result)
	})
}

func TestAddFinalizers(t *testing.T) {
	manager := &Manager{
		finalizers: []string{"finalizer1", "finalizer2"},
	}

	t.Run("add finalizers", func(t *testing.T) {
		strs := []string{"finalizer1", "finalizer3"}
		manager.AddFinalizers(strs)
		assert.Equal(t, []string{"finalizer1", "finalizer3", "finalizer2"}, strs)
	})

	t.Run("no finalizers", func(t *testing.T) {
		strs := []string{"finalizer3", "finalizer4"}
		manager.AddFinalizers(strs)
		assert.Equal(t, []string{"finalizer3", "finalizer4", "finalizer1", "finalizer2"}, strs)
	})
}

func TestRemoveFinalizers(t *testing.T) {
	manager := &Manager{
		finalizers: []string{"finalizer1", "finalizer2"},
	}

	t.Run("remove finalizers", func(t *testing.T) {
		strs := []string{"finalizer1", "finalizer3"}
		manager.RemoveFinalizers(strs)
		assert.Equal(t, []string{"finalizer3"}, strs)
	})

	t.Run("no finalizers", func(t *testing.T) {
		strs := []string{"finalizer3", "finalizer4"}
		manager.RemoveFinalizers(strs)
		assert.Equal(t, []string{"finalizer3", "finalizer4"}, strs)
	})
}
