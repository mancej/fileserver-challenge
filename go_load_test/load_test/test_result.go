package load_test

import "net/http"

type TestResult struct {
	response *http.Response
	testType TestType
	fileName string
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

	if tr.TestType() == CONSISTENCY && tr.response.StatusCode == 404 {
		return true
	}

	return 200 <= tr.response.StatusCode && tr.response.StatusCode < 300
}

func (tr *TestResult) WasError() bool {
	if tr.response == nil || tr.err != nil {
		return true
	}

	if tr.TestType() == CONSISTENCY && tr.response.StatusCode == 404 {
		return false
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
	if tr.TestType() == CONSISTENCY {
		return tr.failed || tr.err != nil || tr.WasThrottled()
	}

	return tr.failed || !tr.WasSuccess() || tr.err != nil || tr.WasThrottled()
}

func (tr *TestResult) Was404() bool {
	if tr.response == nil {
		return false
	}

	return tr.response.StatusCode == 404
}

func (tr *TestResult) WasThrottled() bool {
	if tr.response == nil {
		return false
	}

	return tr.response.StatusCode == http.StatusTooManyRequests
}

func (tr *TestResult) TestType() TestType {
	return tr.testType
}

func (tr *TestResult) FileName() string {
	return tr.fileName
}
