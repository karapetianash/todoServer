package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync"

	"github.com/karapetianash/todo-cli"
)

var (
	ErrNoFound     = errors.New("not found")
	ErrInvalidData = errors.New("invalid data")
)

func rootHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		replyError(w, r, http.StatusNotFound, "")
		return
	}

	content := "There's an API here"
	replyTextContent(w, r, http.StatusOK, content)
}

// todoRouter dispatches appropriate replying function form incoming request
func todoRouter(todoFile string, l sync.Locker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		list := &todo.List{}

		l.Lock()
		defer l.Unlock()

		if err := list.Get(todoFile); err != nil {
			replyError(w, r, http.StatusInternalServerError, err.Error())
			return
		}

		if r.URL.Path == "" {
			switch r.Method {
			case http.MethodGet:
				getAllHandler(w, r, list)
			case http.MethodPost:
				addHandler(w, r, list, todoFile)
			default:
				message := "Method not supported"
				replyError(w, r, http.StatusMethodNotAllowed, message)
			}
			return
		}

		id, err := validateID(r.URL.Path, list)
		if err != nil {
			if errors.Is(err, ErrNoFound) {
				replyError(w, r, http.StatusNotFound, err.Error())
				return
			}

			replyError(w, r, http.StatusBadRequest, err.Error())
			return
		}

		switch r.Method {
		case http.MethodGet:
			getOneHandler(w, r, list, id)
		case http.MethodDelete:
			deleteHandler(w, r, list, id, todoFile)
		case http.MethodPatch:
			pathHandler(w, r, list, id, todoFile)
		default:
			message := "Method not supported"
			replyError(w, r, http.StatusMethodNotAllowed, message)
		}
	}
}

// getAllHandler obtains all to-do items
func getAllHandler(w http.ResponseWriter, r *http.Request, list *todo.List) {
	resp := &todoResponse{
		Results: *list,
	}

	replyJSONContent(w, r, http.StatusOK, resp)
}

// getOneHandler replies with a single item
func getOneHandler(w http.ResponseWriter, r *http.Request, list *todo.List, id int) {
	resp := &todoResponse{
		Results: (*list)[id-1 : id],
	}

	replyJSONContent(w, r, http.StatusOK, resp)
}

// deleteHandler deletes item represented by its id
func deleteHandler(w http.ResponseWriter, r *http.Request, list *todo.List, id int, todoFile string) {
	list.Delete(id)
	if err := list.Save(todoFile); err != nil {
		replyError(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	replyTextContent(w, r, http.StatusNoContent, "")
}

// patchHandler completes a specific item
func pathHandler(w http.ResponseWriter, r *http.Request, list *todo.List, id int, todoFile string) {
	q := r.URL.Query()

	if _, ok := q["complete"]; !ok {
		message := "Missing query param 'complete'"
		replyError(w, r, http.StatusBadRequest, message)
		return
	}

	list.Complete(id)
	if err := list.Save(todoFile); err != nil {
		replyError(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	replyTextContent(w, r, http.StatusNoContent, "")
}

// addHandler adds a new item to the list
func addHandler(w http.ResponseWriter, r *http.Request, list *todo.List, todoFile string) {
	item := struct {
		Task string `json:"task"`
	}{}

	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
		message := fmt.Sprintf("Invalid JSON: %v", err)
		replyError(w, r, http.StatusBadRequest, message)
		return
	}

	list.Add(item.Task)
	if err := list.Save(todoFile); err != nil {
		replyError(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	replyTextContent(w, r, http.StatusCreated, "")
}

// validateID ensures the ID provided by user is valid
func validateID(path string, list *todo.List) (int, error) {
	id, err := strconv.Atoi(path)
	if err != nil {
		return 0, fmt.Errorf("%w: Invalid ID: %s", ErrInvalidData, err)
	}

	if id < 1 {
		return 0, fmt.Errorf("%w: Invalid ID: Less than one", ErrInvalidData)
	}

	if id > len(*list) {
		return id, fmt.Errorf("%w: ID %d not found", ErrNoFound, id)
	}

	return id, nil
}
