package event

import (
	"sync"
	"testing"
)

func TestOperationalEmitterSequencesEachAggregateIndependently(t *testing.T) {
	var mu sync.Mutex
	var got []OperationalEnvelope
	emitter := &OperationalEmitter{Sink: FuncSink(func(e Event) {
		mu.Lock()
		defer mu.Unlock()
		got = append(got, *e.Operational)
	})}
	emitter.Emit(OperationalFact{AggregateKind: AggregateTask, AggregateID: "task-1", Kind: TaskCreated})
	emitter.Emit(OperationalFact{AggregateKind: AggregateModelCall, AggregateID: "call-1", Kind: ModelCallPrepared})
	emitter.Emit(OperationalFact{AggregateKind: AggregateTask, AggregateID: "task-1", Kind: TaskRunning})
	if len(got) != 3 || got[0].Aggregate.Sequence != 1 || got[1].Aggregate.Sequence != 1 || got[2].Aggregate.Sequence != 2 {
		t.Fatalf("unexpected aggregate sequences: %#v", got)
	}
}

func TestOperationalEmitterConcurrentDeliveryPreservesAggregateSequence(t *testing.T) {
	const producers = 32
	const perProducer = 64
	got := make([]uint64, 0, producers*perProducer)
	emitter := &OperationalEmitter{Sink: FuncSink(func(value Event) {
		got = append(got, value.Operational.Aggregate.Sequence)
	})}
	var wait sync.WaitGroup
	for producer := 0; producer < producers; producer++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for index := 0; index < perProducer; index++ {
				emitter.Emit(OperationalFact{AggregateKind: AggregateTask, AggregateID: "task-concurrent", Kind: TaskRunning})
			}
		}()
	}
	wait.Wait()
	if len(got) != producers*perProducer {
		t.Fatalf("delivered = %d, want %d", len(got), producers*perProducer)
	}
	for index, sequence := range got {
		if want := uint64(index + 1); sequence != want {
			t.Fatalf("delivery %d sequence = %d, want %d", index, sequence, want)
		}
	}
}

func BenchmarkOperationalEmitterEmit(b *testing.B) {
	emitter := &OperationalEmitter{Sink: Discard}
	fact := OperationalFact{AggregateKind: AggregateTask, AggregateID: "task-benchmark", Kind: TaskRunning}
	b.ReportAllocs()
	for index := 0; index < b.N; index++ {
		emitter.Emit(fact)
	}
}
