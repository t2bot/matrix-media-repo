package metrics

var beforeMetricsCalledFns = make([]func(), 0)

func OnBeforeMetricsRequested(fn func()) {
	beforeMetricsCalledFns = append(beforeMetricsCalledFns, fn)
}
