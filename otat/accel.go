package otat

type accel struct {
	min   Index
	table [][]int
}

func (a *accel) add(cov coverage, item int) {
	min, max := cov.MinMax()

	if min > max {
		return
	}

	if a.table == nil {
		a.min = min
	} else {
		if min < a.min {
			newtable := make([][]int, Index(len(a.table))+a.min-min)
			copy(newtable[a.min-min:Index(len(a.table))+a.min-min], a.table)
			a.table = newtable
			a.min = min
		}
	}

	if max-a.min >= Index(len(a.table)) {
		newtable := make([][]int, max-a.min+1)
		copy(newtable[:len(a.table)], a.table)
		a.table = newtable
	}

	if cov.sparse != nil {
		for _, i := range cov.sparse {
			a.append(i, item)
		}
	}
	for j := range cov.rangeStart {
		start := cov.rangeStart[j]
		end := cov.rangeEnd[j]
		for i := start; i <= end; i++ {
			a.append(i, item)
		}
	}
}

func (a *accel) append(i Index, item int) {
	for j := range a.table[i-a.min] {
		if a.table[i-a.min][j] == item {
			return
		}
	}

	a.table[i-a.min] = append(a.table[i-a.min], item)
}

func (a *accel) get(i Index) []int {
	if i < a.min || i >= (a.min+Index(len(a.table))) {
		return nil
	}

	return a.table[i-a.min]
}
