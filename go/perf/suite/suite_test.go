// Copyright 2016 Attic Labs, Inc. All rights reserved.
// Licensed under the Apache License, version 2.0:
// http://www.apache.org/licenses/LICENSE-2.0

package suite

import (
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/attic-labs/noms/go/spec"
	"github.com/attic-labs/noms/go/types"
	"github.com/attic-labs/testify/assert"
)

type testSuite struct {
	PerfSuite
	tempFileName, tempDir                  string
	setupTest, tearDownTest                int
	setupRep, tearDownRep                  int
	setupSuite, tearDownSuite              int
	foo, bar, abc, def, nothing, testimate int
}

func (s *testSuite) TestNonEmptyPaths() {
	assert := s.NewAssert()
	assert.NotEqual("", s.AtticLabs)
	assert.NotEqual("", s.Testdata)
	assert.NotEqual("", s.DatabaseSpec)
}

func (s *testSuite) TestDatabase() {
	assert := s.NewAssert()
	val := types.Bool(true)
	r := s.Database.WriteValue(val)
	assert.True(s.Database.ReadValue(r.TargetHash()).Equals(val))
}

func (s *testSuite) TestTempFile() {
	s.tempFileName = s.TempFile().Name()
	s.tempDir = s.TempDir()
}

func (s *testSuite) TestGlob() {
	assert := s.NewAssert()
	f := s.TempFile()
	f.Close()

	create := func(suffix string) {
		f, err := os.Create(f.Name() + suffix)
		assert.NoError(err)
		f.Close()
	}

	create("a")
	create(".a")
	create(".b")

	glob := s.OpenGlob(f.Name() + ".*")
	assert.Equal(2, len(glob))
	assert.Equal(f.Name()+".a", glob[0].(*os.File).Name())
	assert.Equal(f.Name()+".b", glob[1].(*os.File).Name())

	s.CloseGlob(glob)
	b := make([]byte, 16)
	_, err := glob[0].Read(b)
	assert.Error(err)
	_, err = glob[1].Read(b)
	assert.Error(err)
}

func (s *testSuite) TestPause() {
	s.Pause(func() {
		s.waitForSmidge()
	})
}

func (s *testSuite) TestFoo() {
	s.foo++
	s.waitForSmidge()
}

func (s *testSuite) TestBar() {
	s.bar++
	s.waitForSmidge()
}

func (s *testSuite) Test01Abc() {
	s.abc++
	s.waitForSmidge()
}

func (s *testSuite) Test02Def() {
	s.def++
	s.waitForSmidge()
}

func (s *testSuite) testNothing() {
	s.nothing++
	s.waitForSmidge()
}

func (s *testSuite) Testimate() {
	s.testimate++
	s.waitForSmidge()
}

func (s *testSuite) SetupTest() {
	s.setupTest++
}

func (s *testSuite) TearDownTest() {
	s.tearDownTest++
}

func (s *testSuite) SetupRep() {
	s.setupRep++
}

func (s *testSuite) TearDownRep() {
	s.tearDownRep++
}

func (s *testSuite) SetupSuite() {
	s.setupSuite++
}

func (s *testSuite) TearDownSuite() {
	s.tearDownSuite++
}

func (s *testSuite) waitForSmidge() {
	// Tests should call this to make sure the measurement shows up as > 0, not that it shows up as a millisecond.
	<-time.After(time.Millisecond)
}

func TestSuite(t *testing.T) {
	runTestSuite(t, false)
}

func TestSuiteWithMem(t *testing.T) {
	runTestSuite(t, true)
}

func runTestSuite(t *testing.T, mem bool) {
	assert := assert.New(t)

	// Write test results to our own temporary LDB database.
	ldbDir, err := ioutil.TempDir("", "suite.TestSuite")
	assert.NoError(err)
	defer os.RemoveAll(ldbDir)

	flagVal, repeatFlagVal, memFlagVal := *perfFlag, *perfRepeatFlag, *perfMemFlag
	*perfFlag, *perfRepeatFlag, *perfMemFlag = ldbDir, 3, mem
	defer func() {
		*perfFlag, *perfRepeatFlag, *perfMemFlag = flagVal, repeatFlagVal, memFlagVal
	}()

	s := &testSuite{}
	Run("ds", t, s)

	expectedTests := []string{
		"Abc",
		"Bar",
		"Database",
		"Def",
		"Foo",
		"Glob",
		"NonEmptyPaths",
		"Pause",
		"TempFile",
	}

	// The temp file and dir should have been cleaned up.
	_, err = os.Stat(s.tempFileName)
	assert.NotNil(err)
	_, err = os.Stat(s.tempDir)
	assert.NotNil(err)

	// The correct number of Setup/TearDown calls should have been run.
	assert.Equal(1, s.setupSuite)
	assert.Equal(1, s.tearDownSuite)
	assert.Equal(*perfRepeatFlag, s.setupRep)
	assert.Equal(*perfRepeatFlag, s.tearDownRep)
	assert.Equal(*perfRepeatFlag*len(expectedTests), s.setupTest)
	assert.Equal(*perfRepeatFlag*len(expectedTests), s.tearDownTest)

	// The results should have been written to the "ds" dataset.
	db, ds, err := spec.GetDataset(ldbDir + "::ds")
	defer db.Close()
	assert.NoError(err)
	head := ds.HeadValue().(types.Struct)

	// These tests mostly assert that the structure of the results is correct. Specific values are hard.

	getOrFail := func(s types.Struct, f string) types.Value {
		val, ok := s.MaybeGet(f)
		assert.True(ok)
		return val
	}

	env, ok := getOrFail(head, "environment").(types.Struct)
	assert.True(ok)

	getOrFail(env, "diskUsages")
	getOrFail(env, "cpus")
	getOrFail(env, "mem")
	getOrFail(env, "host")
	getOrFail(env, "partitions")

	// Todo: re-enable this code once demo-server gets build without CodePipeline
	// This fails with CodePipeline because the source code is brought into
	// Jenkins as a zip file rather than as a git repo.
	//nomsRevision := getOrFail(head, "nomsRevision")
	//assert.True(ok)
	//assert.True(string(nomsRevision.(types.String)) != "")
	//getOrFail(head, "testdataRevision")

	reps, ok := getOrFail(head, "reps").(types.List)
	assert.True(ok)
	assert.Equal(*perfRepeatFlag, int(reps.Len()))

	reps.IterAll(func(rep types.Value, _ uint64) {
		i := 0

		rep.(types.Map).IterAll(func(k, timesVal types.Value) {
			if assert.True(i < len(expectedTests)) {
				assert.Equal(expectedTests[i], string(k.(types.String)))
			}

			times := timesVal.(types.Struct)
			assert.True(getOrFail(times, "elapsed").(types.Number) > 0)
			assert.True(getOrFail(times, "total").(types.Number) > 0)

			paused := getOrFail(times, "paused").(types.Number)
			if k == types.String("Pause") {
				assert.True(paused > 0)
			} else {
				assert.True(paused == 0)
			}

			i++
		})

		assert.Equal(i, len(expectedTests))
	})
}

func TestPrefixFlag(t *testing.T) {
	assert := assert.New(t)

	// Write test results to a temporary database.
	ldbDir, err := ioutil.TempDir("", "suite.TestSuite")
	assert.NoError(err)
	defer os.RemoveAll(ldbDir)

	flagVal, prefixFlagVal := *perfFlag, *perfPrefixFlag
	*perfFlag, *perfPrefixFlag = ldbDir, "foo/"
	defer func() {
		*perfFlag, *perfPrefixFlag = flagVal, prefixFlagVal
	}()

	Run("my-prefix/test", t, &PerfSuite{})

	// The results should have been written to "foo/my-prefix/test" not "my-prefix/test".
	db, ds, err := spec.GetDataset(ldbDir + "::my-prefix/test")
	defer db.Close()
	assert.NoError(err)
	_, ok := ds.MaybeHead()
	assert.False(ok)

	db, ds, err = spec.GetDataset(ldbDir + "::foo/my-prefix/test")
	defer db.Close()
	assert.NoError(err)
	_, ok = ds.HeadValue().(types.Struct)
	assert.True(ok)
}

func TestRunFlag(t *testing.T) {
	assert := assert.New(t)

	type expect struct {
		foo, bar, abc, def, nothing, testimate int
	}

	run := func(re string, exp expect) {
		flagVal, memFlagVal, runFlagVal := *perfFlag, *perfMemFlag, *perfRunFlag
		*perfFlag, *perfMemFlag, *perfRunFlag = "mem", true, re
		defer func() {
			*perfFlag, *perfMemFlag, *perfRunFlag = flagVal, memFlagVal, runFlagVal
		}()
		s := testSuite{}
		Run("test", t, &s)
		assert.Equal(exp, expect{s.foo, s.bar, s.abc, s.def, s.nothing, s.testimate})
	}

	run("", expect{foo: 1, bar: 1, abc: 1, def: 1})
	run(".", expect{foo: 1, bar: 1, abc: 1, def: 1})
	run("test", expect{foo: 1, bar: 1, abc: 1, def: 1})
	run("^test", expect{foo: 1, bar: 1, abc: 1, def: 1})
	run("Test", expect{foo: 1, bar: 1, abc: 1, def: 1})
	run("^Test", expect{foo: 1, bar: 1, abc: 1, def: 1})

	run("f", expect{foo: 1, def: 1})
	run("^f", expect{foo: 1})
	run("testf", expect{foo: 1})
	run("^testf", expect{foo: 1})
	run("testF", expect{foo: 1})
	run("^testF", expect{foo: 1})

	run("F", expect{foo: 1, def: 1})
	run("^F", expect{foo: 1})
	run("Testf", expect{foo: 1})
	run("^Testf", expect{foo: 1})
	run("TestF", expect{foo: 1})
	run("^TestF", expect{foo: 1})

	run("ef", expect{def: 1})
	run("def", expect{def: 1})
	run("ddef", expect{})
	run("testdef", expect{})
	run("test01def", expect{})
	run("test02def", expect{def: 1})
	run("Test02def", expect{def: 1})
	run("test02Def", expect{def: 1})
	run("Test02Def", expect{def: 1})

	run("z", expect{})
	run("testz", expect{})
	run("Testz", expect{})

	run("[fa]", expect{foo: 1, bar: 1, abc: 1, def: 1})
	run("[fb]", expect{foo: 1, bar: 1, abc: 1, def: 1})
	run("[fc]", expect{foo: 1, abc: 1, def: 1})
	run("test[fa]", expect{foo: 1})
	run("test[fb]", expect{foo: 1, bar: 1})
	run("test[fc]", expect{foo: 1})
	run("Test[fa]", expect{foo: 1})
	run("Test[fb]", expect{foo: 1, bar: 1})
	run("Test[fc]", expect{foo: 1})

	run("foo|bar", expect{foo: 1, bar: 1})
	run("FOO|bar", expect{foo: 1, bar: 1})
	run("Testfoo|bar", expect{foo: 1, bar: 1})
	run("TestFOO|bar", expect{foo: 1, bar: 1})

	run("Testfoo|Testbar", expect{foo: 1, bar: 1})
	run("TestFOO|Testbar", expect{foo: 1, bar: 1})

	run("footest", expect{})
	run("nothing", expect{})
}
