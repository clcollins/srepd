package agent

import (
	"fmt"
	"time"
)

// ForTestStubIndexWrite disables index file writes by clearing the path.
// This is used only for the 410a §2c revert check.
func (m *SessionManager) ForTestStubIndexWrite() {
	if m.index != nil {
		m.index.path = fmt.Sprintf("/dev/null/nonexistent/%d", time.Now().UnixNano())
	}
}

// IndexEntryCount returns the number of established sessions in the
// index. Exposed for testing only — the decision logic is internal.
func (m *SessionManager) IndexEntryCount() int {
	if m.index == nil {
		return 0
	}
	m.index.mu.Lock()
	defer m.index.mu.Unlock()
	return len(m.index.established)
}
