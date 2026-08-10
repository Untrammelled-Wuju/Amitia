package decision

type GoalRef struct {
	ID       string `json:"id"`
	Revision int64  `json:"revision"`
}

func (r GoalRef) Equals(other GoalRef) bool {
	return r.ID == other.ID && r.Revision == other.Revision
}
