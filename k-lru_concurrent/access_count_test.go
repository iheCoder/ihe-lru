package k_lru_concurrent

import "testing"

func TestBasicUse(t *testing.T) {
	type test struct {
		acm *Acm
		r   []string
	}
	ts := []test{
		{
			acm: &Acm{
				counts: map[string]*accessCount{
					"1": {
						count: 1,
						id:    1,
					},
				},
				k: 0,
			},
			r: []string{},
		},
		{
			acm: &Acm{
				counts: map[string]*accessCount{
					"1": {
						id: 1,
					},
					"2": {
						id: 2,
					},
				},
				k: 1,
			},
			r: []string{"1"},
		},
		{
			acm: &Acm{
				counts: map[string]*accessCount{
					"3": {
						id: 3,
					},
					"1": {
						id: 1,
					},
					"2": {
						id: 2,
					},
				},
				k: 1,
			},
			r: []string{"1"},
		},
		{
			acm: &Acm{
				counts: map[string]*accessCount{
					"3": {
						id: 3,
					},
					"1": {
						id: 1,
					},
					"5": {
						id: 5,
					},
					"7": {
						id: 7,
					},
					"2": {
						id: 2,
					},
				},
				k: 3,
			},
			r: []string{"1", "2", "3"},
		},
	}

	for _, ti := range ts {
		r := ti.acm.TopKByQuickSort()
		if !testExpectedResult(r, ti.r) {
			t.Fatalf("got unexpected result: expected result %v", ti.r)
		}
	}
}

func testExpectedResult(cis []*compareItem, r []string) bool {
	if len(cis) != len(r) {
		return false
	}
	l := len(r)
	found := false

	for i := 0; i < l; i++ {
		k := cis[i].key
		for _, ri := range r {
			if k == ri {
				found = true
				break
			}
		}
		if !found {
			return false
		}
		found = false
	}

	return true
}
