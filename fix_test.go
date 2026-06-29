package goyze_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	goyze "github.com/gomatic/go-yze"
)

func TestApplyEditsReplacesSingleRange(t *testing.T) {
	content := []byte("hello world")
	edits := []goyze.TextEdit{
		{Start: 6, End: 11, NewText: "gophers"},
	}

	got, err := goyze.ApplyEdits(content, edits)

	require.NoError(t, err)
	assert.Equal(t, "hello gophers", string(got))
}

func TestApplyEditsAppliesMultipleNonOverlappingEdits(t *testing.T) {
	content := []byte("the quick brown fox")
	edits := []goyze.TextEdit{
		{Start: 4, End: 9, NewText: "slow"},
		{Start: 16, End: 19, NewText: "dog"},
	}

	got, err := goyze.ApplyEdits(content, edits)

	require.NoError(t, err)
	assert.Equal(t, "the slow brown dog", string(got))
}

func TestApplyEditsIsOrderIndependent(t *testing.T) {
	content := []byte("the quick brown fox")
	edits := []goyze.TextEdit{
		{Start: 16, End: 19, NewText: "dog"},
		{Start: 4, End: 9, NewText: "slow"},
	}

	got, err := goyze.ApplyEdits(content, edits)

	require.NoError(t, err)
	assert.Equal(t, "the slow brown dog", string(got))
}

func TestApplyEditsRejectsOverlappingEdits(t *testing.T) {
	content := []byte("hello world")
	edits := []goyze.TextEdit{
		{Start: 0, End: 5, NewText: "hi"},
		{Start: 3, End: 8, NewText: "x"},
	}

	_, err := goyze.ApplyEdits(content, edits)

	require.Error(t, err)
	assert.True(t, errors.Is(err, goyze.ErrOverlappingEdits))
}

func TestApplyEditsRejectsOutOfBoundsEdit(t *testing.T) {
	content := []byte("short")
	edits := []goyze.TextEdit{
		{Start: 2, End: 99, NewText: "x"},
	}

	_, err := goyze.ApplyEdits(content, edits)

	require.Error(t, err)
	assert.True(t, errors.Is(err, goyze.ErrEditOutOfBounds))
}

func TestApplyEditsRejectsNegativeStart(t *testing.T) {
	content := []byte("hello")
	edits := []goyze.TextEdit{
		{Start: -1, End: 2, NewText: "x"},
	}

	_, err := goyze.ApplyEdits(content, edits)

	require.Error(t, err)
	assert.True(t, errors.Is(err, goyze.ErrEditOutOfBounds))
}

func TestApplyEditsRejectsInvertedRange(t *testing.T) {
	content := []byte("hello")
	edits := []goyze.TextEdit{
		{Start: 4, End: 2, NewText: "x"},
	}

	_, err := goyze.ApplyEdits(content, edits)

	require.Error(t, err)
	assert.True(t, errors.Is(err, goyze.ErrEditOutOfBounds))
}

func TestApplyEditsWithNoEditsReturnsContentUnchanged(t *testing.T) {
	content := []byte("unchanged")

	got, err := goyze.ApplyEdits(content, nil)

	require.NoError(t, err)
	assert.Equal(t, "unchanged", string(got))
}

func TestApplyEditsEqualStartInsertionsAreDeterministic(t *testing.T) {
	content := []byte("ad")
	forward := []goyze.TextEdit{
		{Start: 1, End: 1, NewText: "b"},
		{Start: 1, End: 1, NewText: "c"},
	}
	reverse := []goyze.TextEdit{
		{Start: 1, End: 1, NewText: "c"},
		{Start: 1, End: 1, NewText: "b"},
	}

	gotForward, errForward := goyze.ApplyEdits(content, forward)
	gotReverse, errReverse := goyze.ApplyEdits(content, reverse)

	require.NoError(t, errForward)
	require.NoError(t, errReverse)
	// The total order (NewText breaks a shared Start/End tie) fixes the splice
	// result regardless of the order the caller supplied the edits in.
	assert.Equal(t, "abcd", string(gotForward))
	assert.Equal(t, string(gotForward), string(gotReverse))
}

func TestApplyEditsEqualStartOverlapIsDeterministic(t *testing.T) {
	content := []byte("hello")
	forward := []goyze.TextEdit{
		{Start: 1, End: 3, NewText: "x"},
		{Start: 1, End: 4, NewText: "y"},
	}
	reverse := []goyze.TextEdit{
		{Start: 1, End: 4, NewText: "y"},
		{Start: 1, End: 3, NewText: "x"},
	}

	_, errForward := goyze.ApplyEdits(content, forward)
	_, errReverse := goyze.ApplyEdits(content, reverse)

	// Two replacements sharing a Start always overlap, and the verdict is the
	// same whichever order the caller listed them in.
	require.ErrorIs(t, errForward, goyze.ErrOverlappingEdits)
	require.ErrorIs(t, errReverse, goyze.ErrOverlappingEdits)
}

func TestApplyEditsHandlesPureInsertion(t *testing.T) {
	content := []byte("ac")
	edits := []goyze.TextEdit{
		{Start: 1, End: 1, NewText: "b"},
	}

	got, err := goyze.ApplyEdits(content, edits)

	require.NoError(t, err)
	assert.Equal(t, "abc", string(got))
}
