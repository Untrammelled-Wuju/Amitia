package protocol

type Sequence int64

func (s Sequence) IsValid() bool {
	return s >= 0
}

type SequenceDisposition string

const (
	SequenceDispositionNext      SequenceDisposition = "next"
	SequenceDispositionDuplicate SequenceDisposition = "duplicate"
	SequenceDispositionStale     SequenceDisposition = "stale"
	SequenceDispositionGap       SequenceDisposition = "gap"
)

func ClassifySequence(last, incoming Sequence) SequenceDisposition {
	if incoming < last {
		return SequenceDispositionStale
	}
	if incoming == last {
		return SequenceDispositionDuplicate
	}
	if incoming == last+1 {
		return SequenceDispositionNext
	}
	return SequenceDispositionGap
}

type SequenceError struct {
	Last        Sequence
	Incoming    Sequence
	Disposition SequenceDisposition
}

func (e *SequenceError) Error() string {
	return "sequence error: disposition=" + string(e.Disposition)
}
