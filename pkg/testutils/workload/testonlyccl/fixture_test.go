// Copyright 2018 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

package testonlyccl

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"

	"cloud.google.com/go/storage"
	_ "github.com/cockroachdb/cockroach/pkg/ccl/sqlccl"
	"github.com/cockroachdb/cockroach/pkg/testutils"
	"github.com/cockroachdb/cockroach/pkg/testutils/serverutils"
	"github.com/cockroachdb/cockroach/pkg/testutils/sqlutils"
	"github.com/cockroachdb/cockroach/pkg/testutils/workload"
	"github.com/spf13/pflag"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"

	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/timeutil"
)

const fixtureTestGenRows = 10

type fixtureTestGen struct {
	flags *pflag.FlagSet
	val   string
}

func makeTestWorkload() workload.Generator {
	g := &fixtureTestGen{flags: pflag.NewFlagSet(`fx`, pflag.ContinueOnError)}
	g.flags.StringVar(&g.val, `val`, `default`, `The value for each row`)
	return g
}

func (fixtureTestGen) Meta() workload.Meta              { return workload.Meta{Name: `fixture`} }
func (g fixtureTestGen) Flags() *pflag.FlagSet          { return g.flags }
func (g fixtureTestGen) Configure(flags []string) error { return g.flags.Parse(flags) }
func (fixtureTestGen) Ops() []workload.Operation        { return nil }
func (g fixtureTestGen) Tables() []workload.Table {
	return []workload.Table{{
		Name:            `fx`,
		Schema:          `(key INT PRIMARY KEY, value INT)`,
		InitialRowCount: fixtureTestGenRows,
		InitialRowFn:    func(rowIdx int) []string { return []string{strconv.Itoa(rowIdx), g.val} },
	}}
}

func TestFixture(t *testing.T) {
	defer leaktest.AfterTest(t)()
	ctx := context.Background()

	gcsBucket := os.Getenv(`GS_BUCKET`)
	gcsKey := os.Getenv(`GS_JSONKEY`)
	if gcsBucket == "" || gcsKey == "" {
		t.Skip("GS_BUCKET and GS_JSONKEY env vars must be set")
	}

	// This prevents leaking an http conn goroutine.
	http.DefaultTransport.(*http.Transport).DisableKeepAlives = true
	source, err := google.JWTConfigFromJSON([]byte(gcsKey), storage.ScopeReadWrite)
	if err != nil {
		t.Fatalf(`%+v`, err)
	}
	gcs, err := storage.NewClient(ctx,
		option.WithScopes(storage.ScopeReadWrite),
		option.WithTokenSource(source.TokenSource(ctx)))
	if err != nil {
		t.Fatalf(`%+v`, err)
	}
	defer func() { _ = gcs.Close() }()

	s, db, _ := serverutils.StartServer(t, base.TestServerArgs{})
	defer s.Stopper().Stop(ctx)
	sqlDB := sqlutils.MakeSQLRunner(db)
	sqlDB.Exec(t, `SET CLUSTER SETTING cloudstorage.gs.default.key = $1`, gcsKey)

	gen := makeTestWorkload()
	flag := fmt.Sprintf(`val=%d`, timeutil.Now().UnixNano())
	if err := gen.Configure([]string{`--` + flag}); err != nil {
		t.Fatalf(`%+v`, err)
	}

	store := workload.FixtureStore{
		GCSBucket: gcsBucket,
		GCSPrefix: fmt.Sprintf(`TestFixture-%d`, timeutil.Now().UnixNano()),
	}

	if _, err := workload.GetFixture(ctx, gcs, store, gen); !testutils.IsError(err, `fixture not found`) {
		t.Fatalf(`expected "fixture not found" error but got: %+v`, err)
	}

	fixtures, err := workload.ListFixtures(ctx, gcs, store)
	if err != nil {
		t.Fatalf(`%+v`, err)
	}
	if len(fixtures) != 0 {
		t.Errorf(`expected no fixtures but got: %+v`, fixtures)
	}

	fixture, err := workload.MakeFixture(ctx, sqlDB.DB, gcs, store, gen)
	if err != nil {
		t.Fatalf(`%+v`, err)
	}

	fixtures, err = workload.ListFixtures(ctx, gcs, store)
	if err != nil {
		t.Fatalf(`%+v`, err)
	}
	if len(fixtures) != 1 || !strings.Contains(fixtures[0], flag) {
		t.Errorf(`expected exactly one %s fixture but got: %+v`, flag, fixtures)
	}

	sqlDB.Exec(t, `CREATE DATABASE test`)
	if err := workload.RestoreFixture(ctx, sqlDB.DB, fixture, `test`); err != nil {
		t.Fatalf(`%+v`, err)
	}
	sqlDB.CheckQueryResults(t,
		`SELECT COUNT(*) FROM test.fx`, [][]string{{strconv.Itoa(fixtureTestGenRows)}})
}