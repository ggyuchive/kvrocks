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

package incr

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/apache/kvrocks/tests/gocase/util"
	"github.com/stretchr/testify/require"
)

func TestIncr(t *testing.T) {
	configOptions := []util.ConfigOptions{
		{
			Name:    "txn-context-enabled",
			Options: []string{"yes", "no"},
		},
	}

	configsMatrix, err := util.GenerateConfigsMatrix(configOptions)
	require.NoError(t, err)

	for _, configs := range configsMatrix {
		testIncr(t, configs)
	}
}

func testIncr(t *testing.T, configs util.KvrocksServerConfigs) {
	srv := util.StartServer(t, configs)
	defer srv.Close()
	ctx := context.Background()
	rdb := srv.NewClient()
	defer func() { require.NoError(t, rdb.Close()) }()

	t.Run("INCR against non existing key", func(t *testing.T) {
		require.EqualValues(t, 1, rdb.Incr(ctx, "novar").Val())
		require.EqualValues(t, "1", rdb.Get(ctx, "novar").Val())
	})

	t.Run("INCR against key created by incr itself", func(t *testing.T) {
		require.EqualValues(t, 2, rdb.Incr(ctx, "novar").Val())
	})

	t.Run("INCR against key originally set with SET", func(t *testing.T) {
		require.NoError(t, rdb.Set(ctx, "novar", 100, 0).Err())
		require.EqualValues(t, 101, rdb.Incr(ctx, "novar").Val())
	})

	t.Run("INCR over 32bit value", func(t *testing.T) {
		require.NoError(t, rdb.Set(ctx, "novar", 17179869184, 0).Err())
		require.EqualValues(t, 17179869185, rdb.Incr(ctx, "novar").Val())
	})

	t.Run("INCRBY over 32bit value with over 32bit increment", func(t *testing.T) {
		require.NoError(t, rdb.Set(ctx, "novar", 17179869184, 0).Err())
		require.EqualValues(t, 34359738368, rdb.IncrBy(ctx, "novar", 17179869184).Val())
	})

	t.Run("INCR over an expired key", func(t *testing.T) {
		require.NoError(t, rdb.SetEx(ctx, "expired-foo", "1024", 2*time.Second).Err())
		require.NoError(t, rdb.SetEx(ctx, "expired-str", "value", 2*time.Second).Err())
		time.Sleep(3 * time.Second)
		require.EqualValues(t, 1, rdb.IncrBy(ctx, "expired-foo", 1).Val())
		require.EqualValues(t, 1, rdb.IncrBy(ctx, "expired-str", 1).Val())
	})

	t.Run("INCR fails against key with empty value", func(t *testing.T) {
		require.NoError(t, rdb.Set(ctx, "novar", "", 0).Err())
		util.ErrorRegexp(t, rdb.Incr(ctx, "novar").Err(), "ERR.*")
	})

	t.Run("INCR fails against key with spaces (left)", func(t *testing.T) {
		require.NoError(t, rdb.Set(ctx, "novar", "    11", 0).Err())
		util.ErrorRegexp(t, rdb.Incr(ctx, "novar").Err(), "ERR.*")
	})

	t.Run("INCR fails against key with spaces (right)", func(t *testing.T) {
		require.NoError(t, rdb.Set(ctx, "novar", "11    ", 0).Err())
		util.ErrorRegexp(t, rdb.Incr(ctx, "novar").Err(), "ERR.*")
	})

	t.Run("INCR fails against key with spaces (both)", func(t *testing.T) {
		require.NoError(t, rdb.Set(ctx, "novar", "   11    ", 0).Err())
		util.ErrorRegexp(t, rdb.Incr(ctx, "novar").Err(), "ERR.*")
	})

	t.Run("INCR fails against a key holding a list", func(t *testing.T) {
		require.NoError(t, rdb.RPush(ctx, "mylist", 1).Err())
		require.ErrorContains(t, rdb.Incr(ctx, "mylist").Err(), "WRONGTYPE")
		require.NoError(t, rdb.RPop(ctx, "mylist").Err())
	})

	t.Run("DECRBY over 32bit value with over 32bit increment, negative res", func(t *testing.T) {
		require.NoError(t, rdb.Set(ctx, "novar", 17179869184, 0).Err())
		require.EqualValues(t, -1, rdb.DecrBy(ctx, "novar", 17179869185).Val())
	})

	t.Run("DECRBY negation overflow", func(t *testing.T) {
		require.NoError(t, rdb.Do(ctx, "SET", "foo", "0").Err())
		util.ErrorRegexp(t, rdb.Do(ctx, "DECRBY", "foo", "-9223372036854775808").Err(), ".*decrement would overflow.*")
	})

	t.Run("INCRBYFLOAT against non existing key", func(t *testing.T) {
		require.NoError(t, rdb.Del(ctx, "novar").Err())
		require.EqualValues(t, 1, rdb.IncrByFloat(ctx, "novar", 1.0).Val())
		r, err := rdb.Get(ctx, "novar").Float64()
		require.NoError(t, err)
		require.EqualValues(t, 1, r)
		require.EqualValues(t, 1.25, rdb.IncrByFloat(ctx, "novar", 0.25).Val())
		r, err = rdb.Get(ctx, "novar").Float64()
		require.NoError(t, err)
		require.EqualValues(t, 1.25, r)
	})

	t.Run("INCRBYFLOAT against key originally set with SET", func(t *testing.T) {
		require.NoError(t, rdb.Set(ctx, "novar", 1.5, 0).Err())
		require.EqualValues(t, 3, rdb.IncrByFloat(ctx, "novar", 1.5).Val())
	})

	t.Run("INCRBYFLOAT over 32bit value", func(t *testing.T) {
		require.NoError(t, rdb.Set(ctx, "novar", 17179869184, 0).Err())
		require.EqualValues(t, 17179869185.5, rdb.IncrByFloat(ctx, "novar", 1.5).Val())
	})

	t.Run("INCRBYFLOAT over 32bit value with over 32bit increment", func(t *testing.T) {
		require.NoError(t, rdb.Set(ctx, "novar", 17179869184, 0).Err())
		require.EqualValues(t, 34359738368, rdb.IncrByFloat(ctx, "novar", 17179869184).Val())
	})

	t.Run("INCRBYFLOAT over an expired key", func(t *testing.T) {
		require.NoError(t, rdb.SetEx(ctx, "expired-foo", "10.24", 2*time.Second).Err())
		time.Sleep(3 * time.Second)
		require.EqualValues(t, 1, rdb.IncrBy(ctx, "expired-foo", 1.0).Val())
	})

	t.Run("INCRBYFLOAT fails against key with spaces (left)", func(t *testing.T) {
		require.NoError(t, rdb.Set(ctx, "novar", "    11", 0).Err())
		util.ErrorRegexp(t, rdb.IncrByFloat(ctx, "novar", 1.0).Err(), "ERR.*valid.*")
	})

	t.Run("INCRBYFLOAT fails against key with spaces (right)", func(t *testing.T) {
		require.NoError(t, rdb.Set(ctx, "novar", "11  ", 0).Err())
		util.ErrorRegexp(t, rdb.IncrByFloat(ctx, "novar", 1.0).Err(), "ERR.*valid.*")
	})

	t.Run("INCRBYFLOAT fails against key with spaces (both)", func(t *testing.T) {
		require.NoError(t, rdb.Set(ctx, "novar", "  11  ", 0).Err())
		util.ErrorRegexp(t, rdb.IncrByFloat(ctx, "novar", 1.0).Err(), "ERR.*valid.*")
	})

	t.Run("INCRBYFLOAT fails against a key holding a list", func(t *testing.T) {
		require.NoError(t, rdb.Del(ctx, "mylist").Err())
		require.NoError(t, rdb.RPush(ctx, "mylist", 1).Err())
		require.ErrorContains(t, rdb.IncrByFloat(ctx, "mylist", 1.0).Err(), "WRONGTYPE")
		require.NoError(t, rdb.Del(ctx, "mylist").Err())
	})

	t.Run("INCRBYFLOAT does not allow NaN or Infinity", func(t *testing.T) {
		require.NoError(t, rdb.Set(ctx, "foo", 0, 0).Err())
		util.ErrorRegexp(t, rdb.Do(ctx, "INCRBYFLOAT", "foo", "+inf").Err(), "ERR.*would produce.*")
	})

	t.Run("INCRBYFLOAT decrement", func(t *testing.T) {
		require.NoError(t, rdb.Set(ctx, "foo", 1, 0).Err())
		require.InDelta(t, -0.1, rdb.IncrByFloat(ctx, "foo", -1.1).Val(), util.DefaultDelta)
	})

	t.Run("string to double with null terminator", func(t *testing.T) {
		require.NoError(t, rdb.Set(ctx, "foo", 1, 0).Err())
		require.NoError(t, rdb.SetRange(ctx, "foo", 2, "2").Err())
		util.ErrorRegexp(t, rdb.IncrByFloat(ctx, "foo", 1.0).Err(), "ERR.*valid.*")
	})

	increx := func(args ...any) []int64 {
		vals, err := rdb.Do(ctx, append([]any{"INCREX"}, args...)...).Int64Slice()
		require.NoError(t, err)
		return vals
	}

	// The reply holds bulk strings in RESP2 and doubles in RESP3.
	increxFloat := func(args ...any) []float64 {
		vals, err := rdb.Do(ctx, append([]any{"INCREX"}, args...)...).Slice()
		require.NoError(t, err)
		floats := make([]float64, 0, len(vals))
		for _, v := range vals {
			switch v := v.(type) {
			case float64:
				floats = append(floats, v)
			case string:
				f, err := strconv.ParseFloat(v, 64)
				require.NoError(t, err)
				floats = append(floats, f)
			default:
				require.Failf(t, "unexpected reply type", "%T", v)
			}
		}
		return floats
	}

	t.Run("INCREX without options increments by 1", func(t *testing.T) {
		require.NoError(t, rdb.Del(ctx, "foo").Err())
		require.Equal(t, []int64{1, 1}, increx("foo"))
		require.NoError(t, rdb.Set(ctx, "foo", 10, 0).Err())
		require.Equal(t, []int64{11, 1}, increx("foo"))
	})

	t.Run("INCREX BYINT positive and negative increments", func(t *testing.T) {
		require.NoError(t, rdb.Set(ctx, "foo", 100, 0).Err())
		require.Equal(t, []int64{105, 5}, increx("foo", "BYINT", 5))
		require.Equal(t, []int64{95, -10}, increx("foo", "BYINT", -10))
		require.Equal(t, []int64{95, 0}, increx("foo", "BYINT", 0))
	})

	t.Run("INCREX BYINT rejects out-of-range results by default", func(t *testing.T) {
		require.NoError(t, rdb.Set(ctx, "foo", 10, 0).Err())
		require.Equal(t, []int64{10, 0}, increx("foo", "BYINT", 100, "UBOUND", 15, "EX", 100))
		require.EqualValues(t, "10", rdb.Get(ctx, "foo").Val())
		require.EqualValues(t, -1, rdb.TTL(ctx, "foo").Val())
		require.Equal(t, []int64{10, 0}, increx("foo", "BYINT", -100, "LBOUND", 0))

		require.NoError(t, rdb.Set(ctx, "foo", 9223372036854775800, 0).Err())
		require.Equal(t, []int64{9223372036854775800, 0}, increx("foo", "BYINT", 9223372036854775800))

		require.NoError(t, rdb.Del(ctx, "foo").Err())
		require.Equal(t, []int64{0, 0}, increx("foo", "BYINT", 5, "LBOUND", 10))
		require.EqualValues(t, 0, rdb.Exists(ctx, "foo").Val())
	})

	t.Run("INCREX BYINT with SATURATE clamps to the bounds", func(t *testing.T) {
		require.NoError(t, rdb.Set(ctx, "foo", 50, 0).Err())
		require.Equal(t, []int64{80, 30}, increx("foo", "BYINT", 100, "UBOUND", 80, "SATURATE"))
		require.EqualValues(t, "80", rdb.Get(ctx, "foo").Val())
		require.Equal(t, []int64{-10, -90}, increx("foo", "BYINT", -200, "LBOUND", -10, "SATURATE"))
		require.Equal(t, []int64{-5, 5}, increx("foo", "LBOUND", -10, "UBOUND", 10, "BYINT", 5, "SATURATE"))

		require.NoError(t, rdb.Set(ctx, "foo", 50, 0).Err())
		require.Equal(t, []int64{30, -20}, increx("foo", "BYINT", 5, "UBOUND", 30, "SATURATE"))
	})

	t.Run("INCREX BYINT with SATURATE clamps overflow to the type limits", func(t *testing.T) {
		require.NoError(t, rdb.Set(ctx, "foo", 9223372036854775800, 0).Err())
		require.Equal(t, []int64{9223372036854775807, 7}, increx("foo", "BYINT", 100, "SATURATE"))
		require.NoError(t, rdb.Set(ctx, "foo", -9223372036854775800, 0).Err())
		require.Equal(t, []int64{-9223372036854775808, -8}, increx("foo", "BYINT", -100, "SATURATE"))

		require.NoError(t, rdb.Set(ctx, "foo", "-9223372036854775808", 0).Err())
		require.ErrorContains(t, rdb.Do(ctx, "INCREX", "foo", "BYINT", 9223372036854775807,
			"LBOUND", 9223372036854775807, "SATURATE").Err(), "applied increment would overflow")
	})

	t.Run("INCREX BYFLOAT increments and clamps", func(t *testing.T) {
		require.NoError(t, rdb.Set(ctx, "foo", 1.5, 0).Err())
		vals := increxFloat("foo", "BYFLOAT", 0.25)
		require.InDelta(t, 1.75, vals[0], util.DefaultDelta)
		require.InDelta(t, 0.25, vals[1], util.DefaultDelta)

		require.NoError(t, rdb.Set(ctx, "foo", 10, 0).Err())
		vals = increxFloat("foo", "BYFLOAT", 100, "UBOUND", 42.5)
		require.InDelta(t, 10, vals[0], util.DefaultDelta)
		require.InDelta(t, 0, vals[1], util.DefaultDelta)
		vals = increxFloat("foo", "BYFLOAT", 100, "UBOUND", 42.5, "SATURATE")
		require.InDelta(t, 42.5, vals[0], util.DefaultDelta)
		require.InDelta(t, 32.5, vals[1], util.DefaultDelta)
		vals = increxFloat("foo", "BYFLOAT", -100, "LBOUND", -5.5, "SATURATE")
		require.InDelta(t, -5.5, vals[0], util.DefaultDelta)
		require.InDelta(t, -48, vals[1], util.DefaultDelta)
	})

	t.Run("INCREX BYFLOAT rejects Infinity and NaN", func(t *testing.T) {
		require.NoError(t, rdb.Set(ctx, "foo", 0, 0).Err())
		require.ErrorContains(t, rdb.Do(ctx, "INCREX", "foo", "BYFLOAT", "+inf").Err(), "BYFLOAT increment cannot be Infinity")
		require.ErrorContains(t, rdb.Do(ctx, "INCREX", "foo", "BYFLOAT", "nan").Err(), "Increment is not a valid float")

		require.NoError(t, rdb.Set(ctx, "foo", "inf", 0).Err())
		require.ErrorContains(t, rdb.Do(ctx, "INCREX", "foo", "BYFLOAT", 1).Err(), "value cannot be Infinity")
		require.EqualValues(t, "inf", rdb.Get(ctx, "foo").Val())
	})

	t.Run("INCREX sets and keeps the TTL", func(t *testing.T) {
		require.NoError(t, rdb.Del(ctx, "foo").Err())
		require.Equal(t, []int64{5, 5}, increx("foo", "EX", 100, "BYINT", 5))
		util.BetweenValues(t, rdb.TTL(ctx, "foo").Val(), 90*time.Second, 100*time.Second)
		require.Equal(t, []int64{6, 1}, increx("foo"))
		util.BetweenValues(t, rdb.TTL(ctx, "foo").Val(), 90*time.Second, 100*time.Second)

		require.Equal(t, []int64{7, 1}, increx("foo", "PX", 50000))
		util.BetweenValues(t, rdb.TTL(ctx, "foo").Val(), 40*time.Second, 50*time.Second)
		require.Equal(t, []int64{8, 1}, increx("foo", "EXAT", time.Now().Add(200*time.Second).Unix()))
		util.BetweenValues(t, rdb.TTL(ctx, "foo").Val(), 190*time.Second, 200*time.Second)
		require.Equal(t, []int64{9, 1}, increx("foo", "PXAT", time.Now().Add(300*time.Second).UnixMilli()))
		util.BetweenValues(t, rdb.TTL(ctx, "foo").Val(), 290*time.Second, 300*time.Second)

		require.Equal(t, []int64{10, 1}, increx("foo", "PERSIST"))
		require.EqualValues(t, -1, rdb.TTL(ctx, "foo").Val())
	})

	t.Run("INCREX ENX only sets the TTL on keys without one", func(t *testing.T) {
		require.NoError(t, rdb.Set(ctx, "foo", 10, 0).Err())
		require.Equal(t, []int64{11, 1}, increx("foo", "EX", 100, "ENX"))
		util.BetweenValues(t, rdb.TTL(ctx, "foo").Val(), 90*time.Second, 100*time.Second)

		require.NoError(t, rdb.Set(ctx, "foo", 10, 500*time.Second).Err())
		require.Equal(t, []int64{11, 1}, increx("foo", "EX", 10, "ENX"))
		util.BetweenValues(t, rdb.TTL(ctx, "foo").Val(), 490*time.Second, 500*time.Second)

		// A past expire time is ignored too, so the key is not deleted.
		require.Equal(t, []int64{12, 1}, increx("foo", "EXAT", 1, "ENX"))
		require.EqualValues(t, "12", rdb.Get(ctx, "foo").Val())
	})

	t.Run("INCREX with a past expire time deletes the key", func(t *testing.T) {
		require.NoError(t, rdb.Set(ctx, "foo", 10, 0).Err())
		require.Equal(t, []int64{11, 1}, increx("foo", "EXAT", 1))
		require.EqualValues(t, 0, rdb.Exists(ctx, "foo").Val())
		require.Equal(t, []int64{1, 1}, increx("foo", "PXAT", 1))
		require.EqualValues(t, 0, rdb.Exists(ctx, "foo").Val())
	})

	t.Run("INCREX fails against non-numeric values and other types", func(t *testing.T) {
		require.NoError(t, rdb.Set(ctx, "foo", "abc", 0).Err())
		require.ErrorContains(t, rdb.Do(ctx, "INCREX", "foo").Err(), "value is not an integer")
		require.ErrorContains(t, rdb.Do(ctx, "INCREX", "foo", "BYFLOAT", 1.5).Err(), "value is not a valid float")

		require.NoError(t, rdb.Del(ctx, "mylist").Err())
		require.NoError(t, rdb.RPush(ctx, "mylist", 1).Err())
		require.ErrorContains(t, rdb.Do(ctx, "INCREX", "mylist").Err(), "WRONGTYPE")
		require.ErrorContains(t, rdb.Do(ctx, "INCREX", "mylist", "BYFLOAT", 1.5).Err(), "WRONGTYPE")
		require.NoError(t, rdb.Del(ctx, "mylist").Err())
	})

	t.Run("INCREX rejects invalid arguments", func(t *testing.T) {
		for _, args := range [][]any{
			{"FOO"},
			{"BYINT"},
			{"BYINT", 1, "BYFLOAT", 1.5},
			{"BYINT", 1, "BYINT", 2},
			{"LBOUND", 0, "LBOUND", 1},
			{"UBOUND", 9, "UBOUND", 8},
			{"SATURATE", "SATURATE"},
			{"EX", 10, "EX", 20},
			{"EX", 10, "PX", 5000},
			{"EX", 10, "PERSIST"},
			{"PERSIST", "PERSIST"},
			{"PERSIST", "ENX"},
			{"ENX", "ENX", "EX", 10},
		} {
			require.ErrorContains(t, rdb.Do(ctx, append([]any{"INCREX", "foo"}, args...)...).Err(), "syntax error", args)
		}

		require.ErrorContains(t, rdb.Do(ctx, "INCREX").Err(), "wrong number of arguments")
		require.ErrorContains(t, rdb.Do(ctx, "INCREX", "foo", "ENX").Err(), "ENX flag requires an expiration")
		require.ErrorContains(t, rdb.Do(ctx, "INCREX", "foo", "BYINT", "1.5").Err(), "Increment is not an integer")
		require.ErrorContains(t, rdb.Do(ctx, "INCREX", "foo", "BYFLOAT", "abc").Err(), "Increment is not a valid float")
		require.ErrorContains(t, rdb.Do(ctx, "INCREX", "foo", "LBOUND", "1.5").Err(), "LBOUND is not an integer")
		require.ErrorContains(t, rdb.Do(ctx, "INCREX", "foo", "BYINT", 1, "LBOUND", 10, "UBOUND", 5).Err(),
			"LBOUND can't be greater than UBOUND")
		require.ErrorContains(t, rdb.Do(ctx, "INCREX", "foo", "BYFLOAT", 0.5, "LBOUND", 10.5, "UBOUND", 1.5).Err(),
			"LBOUND can't be greater than UBOUND")
		require.Error(t, rdb.Do(ctx, "INCREX", "foo", "EX", 0).Err())
	})
}
