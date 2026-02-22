package tools

// Result is the unified return type from tool execution.
type Result struct {
	ForLLM  string `json:"for_llm"`            // content sent to the LLM
	ForUser string `json:"for_user,omitempty"`  // content shown to the user
	Silent  bool   `json:"silent"`              // suppress user message
	IsError bool   `json:"is_error"`            // marks error
	Async   bool   `json:"async"`               // running asynchronously
	Err     error  `json:"-"`                   // internal error (not serialized)
}

func NewResult(forLLM string) *Result {
	return &Result{ForLLM: forLLM}
}

func SilentResult(forLLM string) *Result {
	return &Result{ForLLM: forLLM, Silent: true}
}

func ErrorResult(message string) *Result {
	return &Result{ForLLM: message, IsError: true}
}

func UserResult(content string) *Result {
	return &Result{ForLLM: content, ForUser: content}
}

func AsyncResult(message string) *Result {
	return &Result{ForLLM: message, Async: true}
}

func (r *Result) WithError(err error) *Result {
	r.Err = err
	return r
}
