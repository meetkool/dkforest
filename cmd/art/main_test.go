package main

import (
	"fmt"
	"strings"
	"testing"
)

func getClassName(code int) string {
	const (
		base  = 26
		alphabet = "abcdefghijklmnopqrstuvwxyz"
	)

	if code < 0 {
		return ""
	}

	var className strings.Builder
	for code > 0 {
		remainder := code % base
		code /= base
		className.WriteByte(alphabet[remainder])
	}

	runes := []rune(className.String())
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}

	return string(runes)
}

func TestGetClassName(t *testing.T) {
	testCases := []struct {
		code     int
		expected string
	}{
		{0, "a"},
		{1, "b"},
		{25, "z"},
		{26, "aa"},
		{27, "ab"},
		{51, "az"},
		{52, "ba"},
		{53, "bb"},
		{77, "bz"},
		{78, "ca"},
		{701, "zz"},
		{702, "aaa"},
		{703, "aab"},
		{727, "aaz"},
		{728, "aba"},
		{753, "abz"},
		{754, "aca"},
		{779, "acz"},
		{780, "ada"},
		{806, "aea"},
		{832, "afa"},
		{1352, "aza"},
		{1377, "azz"},
		{1378, "baa"},
		{2053, "bzz"},
		{2054, "caa"},
