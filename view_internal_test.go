// Copyright 2017 Pilosa Corp.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package pilosa

import (
	"io/ioutil"
	"testing"
	"time"

	"golang.org/x/sync/errgroup"
)

// mustOpenView returns a new instance of View with a temporary path.
func mustOpenView(index, field, name string) *view {
	path, err := ioutil.TempDir(*TempDir, "pilosa-view-")
	if err != nil {
		panic(err)
	}

	fo := FieldOptions{
		CacheType: DefaultCacheType,
		CacheSize: DefaultCacheSize,
	}

	v := newView(path, index, field, name, fo)
	if err := v.open(); err != nil {
		panic(err)
	}
	v.rowAttrStore = &memAttrStore{
		store: make(map[uint64]map[string]interface{}),
	}
	return v
}

// Ensure view can open and retrieve a fragment.
func TestView_DeleteFragment(t *testing.T) {
	v := mustOpenView("i", "f", "v")
	defer v.close()

	shard := uint64(9)

	// Create fragment.
	fragment, err := v.CreateFragmentIfNotExists(shard)
	if err != nil {
		t.Fatal(err)
	} else if fragment == nil {
		t.Fatal("expected fragment")
	}

	err = v.deleteFragment(shard)
	if err != nil {
		t.Fatal(err)
	}

	if v.Fragment(shard) != nil {
		t.Fatal("fragment still exists in view")
	}

	// Recreate fragment with same shard, verify that the old fragment was not reused.
	fragment2, err := v.CreateFragmentIfNotExists(shard)
	if err != nil {
		t.Fatal(err)
	} else if fragment == fragment2 {
		t.Fatal("failed to create new fragment")
	}
}

// Ensure that simultaneous attempts to grab a new fragment don't clash even
// if the broadcast operation takes a bit of time.
func TestView_CreateFragmentRace(t *testing.T) {
	var creates errgroup.Group
	v := mustOpenView("i", "f", "v")
	defer v.close()

	// Use a broadcaster which intentionally fails.
	v.broadcaster = delayBroadcaster{delay: 10 * time.Millisecond}

	shard := uint64(0)

	creates.Go(func() error {
		_, err := v.CreateFragmentIfNotExists(shard)
		return err
	})
	creates.Go(func() error {
		_, err := v.CreateFragmentIfNotExists(shard)
		return err
	})
	err := creates.Wait()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// delayBroadcaster is a nopBroadcaster with a configurable delay.
type delayBroadcaster struct {
	nopBroadcaster
	delay time.Duration
}

// SendSync is an implementation of Broadcaster SendSync which delays for a
// specified interval before succeeding.
func (d delayBroadcaster) SendSync(Message) error {
	time.Sleep(d.delay)
	return nil
}
