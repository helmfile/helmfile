package flags

// MockFlagHandler implements FlagHandler for testing
type MockFlagHandler struct {
	handledFlags map[string]interface{}
	changedFlags map[string]bool
}

func NewMockFlagHandler() *MockFlagHandler {
	return &MockFlagHandler{
		handledFlags: make(map[string]interface{}),
		changedFlags: make(map[string]bool),
	}
}

func (h *MockFlagHandler) HandleFlag(name string, value interface{}, changed bool) bool {
	h.handledFlags[name] = value
	h.changedFlags[name] = changed
	return true
}
