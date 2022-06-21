package metrics

import (
	"fmt"
	"github.com/brodyxchen/vsock-sdk/log"
	"sort"
	"strings"
	"time"
)

func LogRoutine(title string, r Registry, freq time.Duration, closeChan chan struct{}) {
	go func() {
		ticker := time.NewTicker(freq)
		for {
			select {
			case _, ok := <-closeChan:
				if !ok {
					return
				}
			case <-ticker.C:
				msg := format(title, r)
				log.Info(msg)
			}
		}
	}()
}

func format(title string, r Registry) string {
	counterList := make([]string, 0)
	gaugeList := make([]string, 0)
	histList := make([]string, 0)
	meterList := make([]string, 0)

	counterCount := 0
	gaugeCount := 0
	histCount := 0
	meterCount := 0

	r.Each(func(name string, i interface{}) {
		switch metric := i.(type) {
		case Counter:
			counterCount++

			number := metric.Count()
			counterList = append(counterList, fmt.Sprintf("%s: %d", name, number))
			//if number != 0 {
			//	metric.Clear()
			//}
		case Gauge:
			gaugeCount++

			number := metric.Value()
			if number != 0 {
				gaugeList = append(gaugeList, fmt.Sprintf("%s: %d", name, number))
			}
		case GaugeFloat64:
			gaugeCount++

			number := metric.Value()
			if number != 0 {
				gaugeList = append(gaugeList, fmt.Sprintf("%s: %f", name, number))
			}
		case Histogram:
			histCount++

			number := metric.Count()
			if number != 0 {
				h := metric.Snapshot()
				metric.Clear() // 清空一下
				ps := h.Percentiles([]float64{0.5, 0.75, 0.95, 0.99})
				histList = append(histList, fmt.Sprintf("%s: count=%d, min=%d, max=%d, mean=%.2f, stddev=%.2f, median=%.2f, 75%%=%.2f, 95%%=%.2f, 99%%=%.2f",
					name, h.Count(), h.Min(), h.Max(), h.Mean(), h.StdDev(), ps[0], ps[1], ps[2], ps[3]))
			}
		case Meter:
			meterCount++

			number := metric.Count()
			if number != 0 {
				m := metric.Snapshot()
				meterList = append(meterList, fmt.Sprintf("%s: count=%d, 1mRate=%.2f, 5mRate=%.2f, 15mRate=%.2f, meanRate=%.2f",
					name, metric.Count(), m.Rate1(), m.Rate5(), m.Rate15(), m.RateMean()))
			}
		}
	})

	sb := strings.Builder{}
	if counterCount > 0 {
		sb.WriteString(fmt.Sprintf("counter(%v):{", counterCount))

		sort.Strings(counterList)
		for _, v := range counterList {
			sb.WriteString("[")
			sb.WriteString(v)
			sb.WriteString("],")
		}

		sb.WriteString("}, ")
	}
	if gaugeCount > 0 {

		sb.WriteString(fmt.Sprintf("gauge(%v):{", gaugeCount))

		sort.Strings(gaugeList)
		for _, v := range gaugeList {
			sb.WriteString("[")
			sb.WriteString(v)
			sb.WriteString("],")
		}

		sb.WriteString("}, ")
	}
	if histCount > 0 {
		sb.WriteString(fmt.Sprintf("hist(%v):{", histCount))

		sort.Strings(histList)
		for _, v := range histList {
			sb.WriteString("[")
			sb.WriteString(v)
			sb.WriteString("],")
		}

		sb.WriteString("}, ")
	}
	if meterCount > 0 {
		sb.WriteString(fmt.Sprintf("meter(%v):{", meterCount))

		sort.Strings(meterList)
		for _, v := range meterList {
			sb.WriteString("[")
			sb.WriteString(v)
			sb.WriteString("],")
		}

		sb.WriteString("}, ")
	}

	if sb.Len() > 0 {
		return title + "==>" + sb.String()
	}

	return sb.String()
}
