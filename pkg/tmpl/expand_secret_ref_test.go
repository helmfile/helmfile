package tmpl

import (
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func Test_fetchSecretValue(t *testing.T) {
	controller := gomock.NewController(t)
	defer controller.Finish()
	c := NewMockvalClient(controller)
	secretsClient = c

	secretPath := "ref+vault://key/#path"
	expectArg := make(map[string]interface{})
	expectArg["key"] = secretPath

	valsResult := make(map[string]interface{})
	valsResult["key"] = "key_value"
	c.EXPECT().Eval(expectArg).Return(valsResult, nil)
	result, err := fetchSecretValue(secretPath)
	assert.Nil(t, err)
	assert.Equal(t, result, "key_value")
}

func Test_fetchSecretValue_error(t *testing.T) {
	controller := gomock.NewController(t)
	defer controller.Finish()
	c := NewMockvalClient(controller)
	secretsClient = c

	secretPath := "ref+vault://key/#path"
	expectArg := make(map[string]interface{})
	expectArg["key"] = secretPath

	expectedErr := errors.New("some error")
	c.EXPECT().Eval(expectArg).Return(nil, expectedErr)
	result, err := fetchSecretValue(secretPath)
	assert.Equal(t, err, expectedErr)
	assert.Equal(t, result, "")
}

func Test_fetchSecretValue_no_key(t *testing.T) {
	controller := gomock.NewController(t)
	defer controller.Finish()
	c := NewMockvalClient(controller)
	secretsClient = c

	secretPath := "ref+vault://key/#path"
	expectArg := make(map[string]interface{})
	expectArg["key"] = secretPath

	valsResult := make(map[string]interface{})
	c.EXPECT().Eval(expectArg).Return(valsResult, nil)
	result, err := fetchSecretValue(secretPath)
	assert.Error(t, err, "unexpected error occurred, map[] doesn't have 'key' key")
	assert.Equal(t, result, "")
}

func Test_fetchSecretValue_invalid_type(t *testing.T) {
	controller := gomock.NewController(t)
	defer controller.Finish()
	c := NewMockvalClient(controller)
	secretsClient = c

	secretPath := "ref+vault://key/#path"
	expectArg := make(map[string]interface{})
	expectArg["key"] = secretPath

	valsResult := make(map[string]interface{})
	valsResult["key"] = 10
	c.EXPECT().Eval(expectArg).Return(valsResult, nil)
	result, err := fetchSecretValue(secretPath)
	assert.Error(t, err, "expected 10 to be string")
	assert.Equal(t, result, "")
}
