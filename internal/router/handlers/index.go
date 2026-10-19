package handlers

import "net/http"

type IndexHandler struct{}

func (h *IndexHandler) GetIndex(response http.ResponseWriter, request *http.Request) {
	response.WriteHeader(http.StatusOK)

	_, err := response.Write([]byte(`Hello from the Gophermart!`))
	if err != nil {
		http.Error(response, err.Error(), http.StatusBadRequest)
		return
	}
}

func NewIndexHandler() *IndexHandler {
	return &IndexHandler{}
}
