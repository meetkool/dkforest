package main

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGetClassName(t *testing.T) {
	assert.Equal(t, "a", getClassName(0))
	assert.Equal(t, "b", getClassName(1))
	assert.Equal(t, "z", getClassName(25))
	assert.Equal(t, "aa", getClassName(26))
	assert.Equal(t, "ab", getClassName(27))
	assert.Equal(t, "az", getClassName(51))
	assert.Equal(t, "ba", getClassName(52))
	assert.Equal(t, "bb", getClassName(53))
	assert.Equal(t, "bz", getClassName(77))
	assert.Equal(t, "ca", getClassName(78))
	assert.Equal(t, "zz", getClassName(701))
	assert.Equal(t, "aaa", getClassName(702))
	assert.Equal(t, "aab", getClassName(703))
	assert.Equal(t, "aaz", getClassName(727))
	assert.Equal(t, "aba", getClassName(728))
	assert.Equal(t, "abz", getClassName(753))
	assert.Equal(t, "aca", getClassName(754))
	assert.Equal(t, "acz", getClassName(779))
	assert.Equal(t, "ada", getClassName(780))
	assert.Equal(t, "aea", getClassName(806))
	assert.Equal(t, "afa", getClassName(832))
	assert.Equal(t, "aza", getClassName(1352))
	assert.Equal(t, "azz", getClassName(1377))
	assert.Equal(t, "baa", getClassName(1378))
	assert.Equal(t, "bzz", getClassName(2053))
	assert.Equal(t, "caa", getClassName(2054))
}

func TestGetNextClassNameStr(t *testing.T) {
	assert.Equal(t, "a", getNextClassNameStr(""))
	assert.Equal(t, "b", getNextClassNameStr("a"))
	assert.Equal(t, "aaaa", getNextClassNameStr("zzz"))
}
