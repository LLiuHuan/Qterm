package ai

func (m *Manager) appendEventLocked(state *agentRunState, kind, stepID, eventState string) {
	state.cursor++
	state.events = append(state.events, AgentEvent{
		Cursor: state.cursor,
		RunID:  state.run.ID,
		Kind:   kind,
		StepID: stepID,
		State:  eventState,
		At:     m.now(),
	})
	if overflow := len(state.events) - m.maxEvents; overflow > 0 {

		trimmed := make([]AgentEvent, len(state.events)-overflow)
		copy(trimmed, state.events[overflow:])
		state.events = trimmed
	}
	if len(state.events) == 0 {
		state.earliest = state.cursor + 1
		return
	}
	state.earliest = state.events[0].Cursor
}

func (m *Manager) snapshotLocked(state *agentRunState, since int) AgentSnapshot {
	if since < 0 {
		since = 0
	}
	run := state.run
	run.Steps = cloneSteps(state.run.Steps)

	if state.run.Context != nil {
		breakdown := *state.run.Context
		run.Context = &breakdown
	}
	events := make([]AgentEvent, 0, len(state.events))
	for _, event := range state.events {
		if event.Cursor > since {
			events = append(events, event)
		}
	}
	return AgentSnapshot{
		Run:            run,
		Events:         events,
		Cursor:         state.cursor,
		EarliestCursor: state.earliest,

		Truncated: since < state.earliest-1,
	}
}
