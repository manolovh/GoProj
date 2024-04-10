package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsValidKeyword(t *testing.T) {
	validKeywords := []string{"plus", "minus", "multiplied", "divided"}
	invalidKeywords := []string{"keyword", "is", "by", "What"}

	for _, kw := range validKeywords {
		assert.True(t, isValidKeyword(kw), "%s should be a valid keyword", kw)
	}

	for _, kw := range invalidKeywords {
		assert.False(t, isValidKeyword(kw), "%s should not be a valid keyword", kw)
	}
}

func TestParseNum(t *testing.T) {
	s := "42"
	var expected float64 = 42

	result := parseNum(s)
	assert.Equal(t, result, expected, "Expected %f, but got %f", expected, result)
}

func TestValidateExpression(t *testing.T) {
	expectedAReason := "Expected a reason, but got an empty string"

	validExpression := "What is 2 plus 3?"
	valid, reason := validateExpression(validExpression)
	assert.True(t, valid, "Valid expression failed the validation")
	assert.Equal(t, reason, "", "Expected an empty reason, but got %s", reason)

	doubleKeyword := "What is 10 plus plus 20?"
	valid, reason = validateExpression(doubleKeyword)
	assert.False(t, valid, "Invalid expression passed the validation")
	assert.NotEqual(t, reason, "", expectedAReason)

	nonMathQuestion := "What day is it today?"
	valid, reason = validateExpression(nonMathQuestion)
	assert.False(t, valid, "Non-math question passed the validation")
	assert.NotEqual(t, reason, "", expectedAReason)

	divisionByZero := "What is 2 multiplied by 3 divided by 0?"
	valid, reason = validateExpression(divisionByZero)
	assert.False(t, valid, "Division by zero passed the validation")
	assert.NotEqual(t, reason, "", expectedAReason)

	incompleteKeyword := "What is 15 divided 3?"
	valid, reason = validateExpression(incompleteKeyword)
	assert.False(t, valid, "Division with incomplete keyword passed the validation")
	assert.NotEqual(t, reason, "", expectedAReason)

	notANum := "What is three plus 5?"
	valid, reason = validateExpression(notANum)
	assert.False(t, valid, "Expression with invalid number passed the validation")
	assert.NotEqual(t, reason, "", expectedAReason)

	invalidExpr := "What is three plus 5"
	valid, reason = validateExpression(invalidExpr)
	assert.False(t, valid, "Invalid expression passed the validation")
	assert.NotEqual(t, reason, "", expectedAReason)

	invalidExpr = "what is 5 multiplied plus 3?"
	valid, reason = validateExpression(invalidExpr)
	assert.False(t, valid, "Invalid expression passed the validation")
	assert.NotEqual(t, reason, "", expectedAReason)

	invalidExpr = "what is 5 plus 3?abc"
	valid, reason = validateExpression(invalidExpr)
	assert.False(t, valid, "Invalid expression passed the validation")
	assert.NotEqual(t, reason, "", expectedAReason)
}

func TestEvaluateExpression(t *testing.T) {
	validExpression := "What is 5 multiplied by 10?"
	var expectedResult float64 = 50

	result, err := evaluateExpression(validExpression)
	assert.Equal(t, err, "", "Error evaluating a valid expression: %s", err)
	assert.Equal(t, result, expectedResult, "Expected %f, but got %f", expectedResult, result)

	invalidExpression := "What is 2 minus?"
	_, err = evaluateExpression(invalidExpression)
	assert.NotEqual(t, err, nil, "Invalid expression evaluated successfully")
}

func TestEvaluateHandler(t *testing.T) {
	validJSON := `{"expression": "What is 2 plus 3?"}`
	invalidJSON := `{"exp": "What is 2 plus 3?"}`
	router := Router(EvaluateEndpoint, evaluateHandler, POST_REQ)

	req := httptest.NewRequest(POST_REQ, EvaluateEndpoint, strings.NewReader(validJSON))
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	var evalResponse EvaluationResponse
	err := json.NewDecoder(res.Body).Decode(&evalResponse)
	assert.Equal(t, evalResponse.Result, "5", "Expected result 5, but got %s", evalResponse.Result)
	assert.Nil(t, err, "Expected valid result, but got", err)
	assert.Equal(t, res.Code, http.StatusOK, "Expected Status 200, but got %d", res.Code)

	req = httptest.NewRequest(POST_REQ, EvaluateEndpoint, strings.NewReader(invalidJSON))
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)

	var errorResp EvaluationResponse
	err = json.NewDecoder(res.Body).Decode(&errorResp)
	assert.Equal(t, errorResp.Result, InvalidJSONError, "Expected message: %s, but got message: %s", InvalidJSONError, errorResp.Result)
	assert.Equal(t, res.Code, http.StatusBadRequest, "Expected Status 400, but got %d", res.Code)
}

func TestValidateHandler(t *testing.T) {
	validJSON := `{"expression": "What is 10 divided by 5 multiplied by 2?"}`
	invalidJSON := `{"exp": "What is 10 divided by 5 multiplied by 2?"}`
	router := Router(ValidateEndpoint, validateHandler, POST_REQ)

	req := httptest.NewRequest(POST_REQ, ValidateEndpoint, strings.NewReader(validJSON))
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	var problemExpr ProblemExpression
	err := json.NewDecoder(res.Body).Decode(&problemExpr)
	assert.Nil(t, err, "Expected valid result, but got", err)
	assert.Equal(t, res.Code, http.StatusOK, "Expected Status 200, but got %d", res.Code)

	req = httptest.NewRequest(POST_REQ, ValidateEndpoint, strings.NewReader(invalidJSON))
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)

	var errorResp ValidationResponse
	_ = json.NewDecoder(res.Body).Decode(&errorResp)
	assert.Equal(t, errorResp.Reason, InvalidJSONError, "Expected message: %s, but got message: %s", InvalidJSONError, errorResp.Reason)
	assert.Equal(t, res.Code, http.StatusBadRequest, "Expected Status 400, but got %d", res.Code)
}

func TestErrorsHandler(t *testing.T) {
	validJSON := `{"expression": "What is 100 minus?"}`
	const loops = 5
	router := Router(EvaluateEndpoint, evaluateHandler, POST_REQ)

	for i := 0; i < loops; i++ {
		postReq := httptest.NewRequest(POST_REQ, EvaluateEndpoint, strings.NewReader(validJSON))
		postRes := httptest.NewRecorder()
		router.ServeHTTP(postRes, postReq)
	}

	for i := 0; i < loops; i++ {
		postReq := httptest.NewRequest(POST_REQ, ValidateEndpoint, strings.NewReader(validJSON))
		postRes := httptest.NewRecorder()
		router.ServeHTTP(postRes, postReq)
	}

	getReq, _ := http.NewRequest(GET_REQ, ErrorsEndpoint, nil)
	getRes := httptest.NewRecorder()

	Router(ErrorsEndpoint, errorsHandler, GET_REQ).ServeHTTP(getRes, getReq)

	var errorsInfo []ErrorInfo
	json.NewDecoder(getRes.Body).Decode(&errorsInfo)

	for _, errors := range errorsInfo {
		if errors.Endpoint == EvaluateEndpoint {
			assert.Equal(t, errors.Frequency, loops,
				"Expected frequency of %s endpoint %d, but got %d", EvaluateEndpoint, loops, errors.Frequency)
		} else if errors.Endpoint == ValidateEndpoint {
			assert.Equal(t, errors.Frequency, loops,
				"Expected frequency of %s endpoint %d, but got %d", ValidateEndpoint, loops, errors.Frequency)
		} else {
			t.Errorf("Expected valid endpoint, but got %s", errors.Endpoint)
		}
	}
}
