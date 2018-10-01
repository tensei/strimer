package main

import (
	"log"
	"testing"
)

func Test_createPrerollAss(t *testing.T) {
	tests := []string{"hmmmm", "fffff"}
	for _, tt := range tests {
		t.Run(tt, func(t *testing.T) {
			got, err := createPrerollAss(tt)
			if err != nil {
				t.Errorf("createPrerollAss() error = %v", err)
				return
			}
			log.Println(got)
		})
	}
}

func Test_createPreroll(t *testing.T) {
	tests := []string{"gmmmmm"}
	for _, tt := range tests {
		t.Run(tt, func(t *testing.T) {
			_, err := createPreroll(tt)
			if err != nil {
				t.Errorf("createPrerollAss() error = %v", err)
			}
		})
	}
}
