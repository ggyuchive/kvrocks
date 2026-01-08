/*
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements.  See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership.  The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License.  You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package util

import (
	"cmp"
	"testing"

	"github.com/stretchr/testify/require"
)

func ErrorRegexp(t testing.TB, err error, rx interface{}, msgAndArgs ...interface{}) {
	require.Error(t, err, msgAndArgs...)
	require.Regexp(t, rx, err.Error(), msgAndArgs...)
}

// BetweenValues asserts start <= d <= end
func BetweenValues[T cmp.Ordered](t testing.TB, d, start, end T, msgAndArgs ...interface{}) {
	require.GreaterOrEqual(t, d, start, msgAndArgs...)
	require.LessOrEqual(t, d, end, msgAndArgs...)
}

// BetweenValuesEx asserts start < d < end
func BetweenValuesEx[T cmp.Ordered](t testing.TB, d, start, end T, msgAndArgs ...interface{}) {
	require.Greater(t, d, start, msgAndArgs...)
	require.Less(t, d, end, msgAndArgs...)
}

func RetryEventually(t testing.TB, condition func() bool, maxAttempts int, msgAndArgs ...interface{}) {
	require.Greater(t, maxAttempts, 0, msgAndArgs...)
	for range maxAttempts {
		if condition() {
			return
		}
	}
	require.Fail(t, "Condition never satisfied", msgAndArgs...)
}
