package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
)

type MessageType struct {
	Content  string `json:content`
	Endpoint string `json:endpoint`
}

type MessageInfo struct {
	Frequency int    `json:frequency`
	ErrorType string `json:errorType`
}

type ProblemExpression struct {
	Expression string `json:"expression"`
}

type ErrorResponse struct {
	Message string `json:"error"`
}

type EvaluationResponse struct {
	Result string `json:"result"`
}

type ValidationResponse struct {
	Valid  bool   `json:"valid"`
	Reason string `json:"reason,omitempty"`
}

type ErrorInfo struct {
	Expression string `json:"expression"`
	Endpoint   string `json:"endpoint"`
	Frequency  int    `json:"frequency"`
	Type       string `json:"type"`
}

const EvaluateEndpoint = "/evaluate"
const ValidateEndpoint = "/validate"
const ErrorsEndpoint = "/errors"
const Port = ":8080"
const GET_REQ = "GET"
const POST_REQ = "POST"

const KeywordPlus = "plus"
const KeywordMinus = "minus"
const KeywordMultiplied = "multiplied"
const KeywordDivided = "divided"
const KeywordBy = "by"
const KeywordWhat = "what"
const KeywordIs = "is"

const InvalidJSONError = "Invalid JSON data"
const InvalidExpressionError = "Invalid expression"
const UnsupportedOperationError = "Unsupported operation"
const NonMathQuestionError = "Non-math question"

var evaluationErrors map[MessageType]MessageInfo = make(map[MessageType]MessageInfo)

func Router(path string, f func(http.ResponseWriter, *http.Request), method string) *mux.Router {
	router := mux.NewRouter()
	router.HandleFunc(path, f).Methods(method)
	return router
}

func beautifyJson(value any) string {
	res, _ := json.MarshalIndent(value, "", "    ")
	return string(res)
}

func isValidKeyword(kw string) bool {
	validKeywords := []string{KeywordPlus, KeywordMinus, KeywordMultiplied, KeywordDivided}
	for _, word := range validKeywords {
		if word == kw {
			return true
		}
	}

	return false
}

func parseNum(s string) float64 {
	num, _ := strconv.Atoi(s)
	return float64(num)
}

func validateExpression(expression string) (bool, string) {
	if len(expression) == 0 || !strings.HasSuffix(expression, "?") {
		return false, InvalidExpressionError
	}

	expression = expression[:len(expression)-1]
	expression = strings.ToLower(expression)
	fields := strings.Fields(expression)

	if len(fields) < 3 || fields[0] != KeywordWhat || fields[1] != KeywordIs {
		return false, NonMathQuestionError
	}

	_, err := strconv.Atoi(fields[2])
	if err != nil {
		// First operand is invalid
		return false, InvalidExpressionError
	}

	for i := 3; i < len(fields); i += 2 {
		if !isValidKeyword(fields[i]) {
			return false, UnsupportedOperationError
		} else if fields[i] == KeywordMultiplied || fields[i] == KeywordDivided {
			if i+1 >= len(fields) || fields[i+1] != KeywordBy {
				// "divided" or "multiplied" not followed by "by"
				return false, InvalidExpressionError
			}

			if i+2 < len(fields) && fields[i] == KeywordDivided && fields[i+1] == KeywordBy {
				num, err := strconv.Atoi(fields[i+2])
				if err == nil && num == 0 {
					// Division by 0
					return false, InvalidExpressionError
				}
			}

			i++
		}

		if i+1 == len(fields) {
			// Number not present after a keyword
			return false, InvalidExpressionError
		}
		_, err := strconv.Atoi(fields[i+1])
		if err != nil {
			// Second operand is invalid
			return false, InvalidExpressionError
		}
	}

	return true, ""
}

func evaluateExpression(expression string) (float64, string) {
	valid, reason := validateExpression(expression)

	if !valid {
		return 0, reason
	}

	expression = expression[:len(expression)-1]
	expression = strings.ToLower(expression)
	fields := strings.Fields(expression)

	firstNum := fields[2]
	result := parseNum(firstNum)

	for i := 3; i < len(fields); i += 2 {
		if i+1 >= len(fields) {
			return 0, InvalidExpressionError
		}

		switch fields[i] {
		case KeywordPlus:
			nextNum := fields[i+1]
			result += parseNum(nextNum)
		case KeywordMinus:
			nextNum := fields[i+1]
			result -= parseNum(nextNum)
		case KeywordMultiplied:
			nextNum := fields[i+2]
			result *= parseNum(nextNum)
			i++
		case KeywordDivided:
			nextNum := fields[i+2]
			result /= parseNum(nextNum)
			i++
		default:
			return 0, UnsupportedOperationError
		}
	}

	return result, ""
}

func evaluateHandler(response http.ResponseWriter, request *http.Request) {
	var problemExpr ProblemExpression
	json.NewDecoder(request.Body).Decode(&problemExpr)

	if reflect.DeepEqual(problemExpr, ProblemExpression{}) {
		response.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(response).Encode(EvaluationResponse{Result: InvalidJSONError})
		return
	}

	result, message := evaluateExpression(problemExpr.Expression)
	if message != "" {
		json.NewEncoder(response).Encode(EvaluationResponse{Result: message})
		key := MessageType{Content: problemExpr.Expression, Endpoint: EvaluateEndpoint}
		_, exists := evaluationErrors[key]

		info := MessageInfo{Frequency: 1, ErrorType: message}
		if exists {
			info = evaluationErrors[key]
			info.Frequency += 1
		}

		evaluationErrors[key] = info
		return
	}

	json.NewEncoder(response).Encode(EvaluationResponse{Result: strconv.FormatFloat(result, 'f', -1, 64)})
}

func validateHandler(response http.ResponseWriter, request *http.Request) {
	var problemExpr ProblemExpression
	json.NewDecoder(request.Body).Decode(&problemExpr)

	if reflect.DeepEqual(problemExpr, ProblemExpression{}) {
		response.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(response).Encode(ValidationResponse{Valid: false, Reason: InvalidJSONError})
		return
	}

	valid, reason := validateExpression(problemExpr.Expression)
	json.NewEncoder(response).Encode(ValidationResponse{Valid: valid, Reason: reason})

	if reason != "" {
		key := MessageType{Content: problemExpr.Expression, Endpoint: ValidateEndpoint}
		_, exists := evaluationErrors[key]

		info := MessageInfo{Frequency: 1, ErrorType: reason}
		if exists {
			info = evaluationErrors[key]
			info.Frequency += 1
		}

		evaluationErrors[key] = info
	}
}

func errorsHandler(response http.ResponseWriter, request *http.Request) {
	var errorsInfo []ErrorInfo
	for message, info := range evaluationErrors {
		errorInfo := ErrorInfo{
			Expression: message.Content,
			Endpoint:   message.Endpoint,
			Frequency:  info.Frequency,
			Type:       info.ErrorType,
		}
		errorsInfo = append(errorsInfo, errorInfo)
	}

	json.NewEncoder(response).Encode(errorsInfo)
}

func main() {
	router := mux.NewRouter()

	router.HandleFunc(EvaluateEndpoint, evaluateHandler)
	router.HandleFunc(ValidateEndpoint, validateHandler)
	router.HandleFunc(ErrorsEndpoint, errorsHandler)

	go func() {
		for {
			message := ""

			fmt.Print("Enter <endpoint> <file, when-needed>, \"exit\" to leave: ")
			scanner := bufio.NewScanner(os.Stdin)
			scanner.Scan()
			if scanner.Err() != nil || scanner.Text() == "" {
				fmt.Println("Try again..")
				continue
			}

			userInput := scanner.Text()
			commands := strings.Split(userInput, " ")

			if commands[0] == "exit" {
				break
			}

			if len(commands) == 1 {
				getReq, _ := http.NewRequest(GET_REQ, ErrorsEndpoint, nil)
				getRes := httptest.NewRecorder()

				Router(ErrorsEndpoint, errorsHandler, GET_REQ).ServeHTTP(getRes, getReq)
				var errorsInfo []ErrorInfo
				json.NewDecoder(getRes.Body).Decode(&errorsInfo)

				message = beautifyJson(errorsInfo)
			} else if len(commands) == 2 {
				jsonFile, err := os.Open(commands[1])
				if err != nil {
					message = err.Error()
				}
				defer jsonFile.Close()

				byteVal, _ := io.ReadAll(jsonFile)

				endpoint := EvaluateEndpoint
				handler := evaluateHandler

				if commands[0] == ValidateEndpoint {
					endpoint = ValidateEndpoint
					handler = validateHandler
				}

				inner_router := Router(endpoint, handler, POST_REQ)
				postReq := httptest.NewRequest(POST_REQ, endpoint, strings.NewReader(string(byteVal)))
				postRes := httptest.NewRecorder()
				inner_router.ServeHTTP(postRes, postReq)

				if endpoint == EvaluateEndpoint {
					var errorResp EvaluationResponse
					err = json.NewDecoder(postRes.Body).Decode(&errorResp)

					message = beautifyJson(errorResp)
				} else if endpoint == ValidateEndpoint {
					var errorResp ValidationResponse
					err = json.NewDecoder(postRes.Body).Decode(&errorResp)

					message = beautifyJson(errorResp)
				}
			} else {
				message = "Unnsuported command list. Try <program_name> <endpoint> <file - optional>"
			}

			fmt.Println(message)
		}
	}()

	http.ListenAndServe(Port, router)
}
