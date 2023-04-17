package load_test

import "net/http"

type TestResult struct {
	response *http.Response
	message  string
	err      error
	failed   bool
}

func NewTestResult(response *http.Response) TestResult {
	return TestResult{response: response}
}

func (tr *TestResult) WasSuccess() bool {
	if tr.response == nil {
		return false
	}

	return 200 <= tr.response.StatusCode && tr.response.StatusCode < 300
}

func (tr *TestResult) WasError() bool {
	if tr.response == nil || tr.err != nil {
		return true
	}

	return tr.response.StatusCode >= 400
}

func (tr *TestResult) Was5XX() bool {
	if tr.response == nil {
		return false
	}

	return tr.response.StatusCode >= 500
}

func (tr *TestResult) WasTestFailure() bool {
	return tr.failed || !tr.WasSuccess() || tr.err != nil
}

func (tr *TestResult) WasThrottled() bool {
	if tr.response == nil {
		return false
	}

	return tr.response.StatusCode == http.StatusTooManyRequests
}
