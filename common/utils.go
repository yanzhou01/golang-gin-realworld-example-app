// Common tools and helper functions
package common

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/golang-jwt/jwt/v5"
)

var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

// A helper function to generate random string
func RandString(n int) string {
	b := make([]rune, n)
	for i := range b {
		randIdx, err := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		if err != nil {
			panic(err)
		}
		b[i] = letters[randIdx.Int64()]
	}
	return string(b)
}

// A helper function to generate random int
func RandInt() int {
	randNum, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		panic(err)
	}
	return int(randNum.Int64())
}

// Keep this two config private, it should not expose to open source
const JWTSecret = "A String Very Very Very Strong!!@##$!@#$"      // #nosec G101
const RandomPassword = "A String Very Very Very Random!!@##$!@#4" // #nosec G101

// A Util function to generate jwt_token which can be used in the request header
func GenToken(id uint) string {
	jwt_token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"id":  id,
		"exp": time.Now().Add(time.Hour * 24).Unix(),
	})
	// Sign and get the complete encoded token as a string
	token, err := jwt_token.SignedString([]byte(JWTSecret))
	if err != nil {
		fmt.Printf("failed to sign JWT token for id %d: %v\n", id, err)
		return ""
	}
	return token
}

// My own Error type that will help return my customized Error info
//
//	{"database": {"hello":"no such table", error: "not_exists"}}
type CommonError struct {
	Errors map[string]interface{} `json:"errors"`
}

func validationMessage(v validator.FieldError) string {
	switch v.Tag() {
	case "required":
		return "can't be blank"
	case "email":
		// Empty string should say "can't be blank" rather than "is invalid"
		if val, ok := v.Value().(string); ok && val == "" {
			return "can't be blank"
		}
		return "is invalid"
	case "min":
		// If the value is an empty string, say "can't be blank" rather than "is too short"
		if val, ok := v.Value().(string); ok && val == "" {
			return "can't be blank"
		}
		return fmt.Sprintf("is too short (minimum is %s characters)", v.Param())
	case "max":
		return fmt.Sprintf("is too long (maximum is %s characters)", v.Param())
	case "url":
		return "is invalid"
	default:
		return fmt.Sprintf("%s failed validation: %s", strings.ToLower(v.Field()), v.Tag())
	}
}

// NewValidatorError returns errors in format: {"errors": {"field": ["message"]}}
func NewValidatorError(err error) CommonError {
	res := CommonError{}
	res.Errors = make(map[string]interface{})
	errs, ok := err.(validator.ValidationErrors)
	if !ok {
		res.Errors["body"] = []string{err.Error()}
		return res
	}
	for _, v := range errs {
		fieldName := strings.ToLower(v.Field())
		res.Errors[fieldName] = []string{validationMessage(v)}
	}
	return res
}

// NewError wraps error in format: {"errors": {"key": ["message"]}}
func NewError(key string, err error) CommonError {
	res := CommonError{}
	res.Errors = make(map[string]interface{})
	res.Errors[key] = []string{err.Error()}
	return res
}

// IsDuplicateKeyError checks if a DB error is a unique constraint violation.
// Returns (true, "username") or (true, "email") etc.
func IsDuplicateKeyError(err error) (bool, string) {
	msg := strings.ToLower(err.Error())
	isDup := strings.Contains(msg, "unique constraint failed") ||
		strings.Contains(msg, "duplicate entry") ||
		strings.Contains(msg, "1062")
	if !isDup {
		return false, ""
	}
	if strings.Contains(msg, "username") {
		return true, "username"
	}
	if strings.Contains(msg, "email") {
		return true, "email"
	}
	if strings.Contains(msg, "slug") {
		return true, "slug"
	}
	return true, "record"
}

// Changed the c.MustBindWith() ->  c.ShouldBindWith().
// I don't want to auto return 400 when error happened.
// origin function is here: https://github.com/gin-gonic/gin/blob/master/context.go
func Bind(c *gin.Context, obj interface{}) error {
	b := binding.Default(c.Request.Method, c.ContentType())
	return c.ShouldBindWith(obj, b)
}
