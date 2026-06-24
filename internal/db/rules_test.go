package db

import (
	"testing"
)

func TestEvaluateRule(t *testing.T) {
	tests := []struct {
		expr    string
		env     map[string]interface{}
		want    bool
		wantErr bool
	}{
		{
			expr: "true",
			env:  nil,
			want: true,
		},
		{
			expr: "false",
			env:  nil,
			want: false,
		},
		{
			expr: "auth != null",
			env: map[string]interface{}{
				"auth": map[string]interface{}{
					"uid": "user123",
				},
			},
			want: true,
		},
		{
			expr: "auth != null",
			env: map[string]interface{}{
				"auth": nil,
			},
			want: false,
		},
		{
			expr: "auth.role == 'admin'",
			env: map[string]interface{}{
				"auth": map[string]interface{}{
					"role": "admin",
				},
			},
			want: true,
		},
		{
			expr: "auth.role == 'admin'",
			env: map[string]interface{}{
				"auth": map[string]interface{}{
					"role": "user",
				},
			},
			want: false,
		},
		{
			expr: "auth.uid == data.user_id",
			env: map[string]interface{}{
				"auth": map[string]interface{}{
					"uid": "123",
				},
				"data": map[string]interface{}{
					"user_id": "123",
				},
			},
			want: true,
		},
		{
			expr: "auth.uid == data.user_id",
			env: map[string]interface{}{
				"auth": map[string]interface{}{
					"uid": "123",
				},
				"data": map[string]interface{}{
					"user_id": "456",
				},
			},
			want: false,
		},
		{
			expr: "auth.uid == data.user_id || auth.role == 'admin'",
			env: map[string]interface{}{
				"auth": map[string]interface{}{
					"uid":  "123",
					"role": "admin",
				},
				"data": map[string]interface{}{
					"user_id": "456",
				},
			},
			want: true,
		},
		{
			expr: "!(auth == null)",
			env: map[string]interface{}{
				"auth": map[string]interface{}{
					"uid": "123",
				},
			},
			want: true,
		},
		{
			expr: "data.age > 18",
			env: map[string]interface{}{
				"data": map[string]interface{}{
					"age": 20.0,
				},
			},
			wantErr: true, // we don't support > operator in rules.go, only == and !=
		},
	}

	for _, tt := range tests {
		got, err := EvaluateRule(tt.expr, tt.env)
		if (err != nil) != tt.wantErr {
			t.Errorf("EvaluateRule(%q) error = %v, wantErr %v", tt.expr, err, tt.wantErr)
			continue
		}
		if err == nil && got != tt.want {
			t.Errorf("EvaluateRule(%q) = %v, want %v", tt.expr, got, tt.want)
		}
	}
}
