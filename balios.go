// Package balios provides the fastest in-memory cache implementation in Go.
//
// Balios is based on W-TinyLFU (Window Tiny Least Frequently Used) algorithm
// and designed to outperform existing solutions like Otter and Ristretto
// through zero-allocation operations and lock-free data structures.
//
// Example usage:
//
//	cache := balios.NewCache(balios.Config{
//		MaxSize: 10_000,
//		WindowRatio: 0.01,
//	})
//
//	cache.Set("key", "value")
//	value, found := cache.Get("key")
//
// Copyright (c) 2025 AGILira - A. Giordano
// Series: an AGILira fragment
// SPDX-License-Identifier: MPL-2.0

package balios

const (
	// Version of Balios cache library
	Version = "v0.1.0-dev"

	// DefaultMaxSize is the default maximum number of entries
	DefaultMaxSize = 10_000

	// DefaultWindowRatio is the default ratio of window cache to total cache size
	DefaultWindowRatio = 0.01 // 1%

	// DefaultCounterBits is the default number of bits per counter in frequency sketch
	DefaultCounterBits = 4
)
