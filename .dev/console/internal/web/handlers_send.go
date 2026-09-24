package web

import (
	"fmt"
	"io"
	"net/http"

	"github.com/eliasmeireles/gomailer/dev/console/internal/message"
	"github.com/eliasmeireles/gomailer/dev/console/internal/service"
)

const maxUploadBytes = 20 << 20

type resultData struct {
	Result  service.SendResult
	Payload string
	// Response is the HTTP source answer rendered as JSON (HTTP channel only).
	Response string
	Error    string
}

func (s *Server) send(w http.ResponseWriter, r *http.Request) {
	form, err := parseForm(r)
	if err != nil {
		s.render(w, "result.html", resultData{Error: err.Error()}, http.StatusBadRequest)
		return
	}

	result, err := s.console.Send(r.Context(), form)
	if err != nil {
		s.render(w, "result.html", resultData{Error: err.Error()}, http.StatusUnprocessableEntity)
		return
	}
	s.render(w, "result.html", resultData{
		Result:   result,
		Payload:  previewJSON(result.Email),
		Response: responseJSON(result.Response),
	}, http.StatusOK)
}

func parseForm(r *http.Request) (message.Form, error) {
	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		return message.Form{}, fmt.Errorf("invalid form: %w", err)
	}

	files, err := readFiles(r)
	if err != nil {
		return message.Form{}, err
	}

	return message.Form{
		Channel:                message.Channel(r.FormValue("channel")),
		ID:                     r.FormValue("id"),
		From:                   r.FormValue("from"),
		To:                     r.FormValue("to"),
		Cc:                     r.FormValue("cc"),
		Bcc:                    r.FormValue("bcc"),
		Subject:                r.FormValue("subject"),
		HTML:                   r.FormValue("html"),
		Format:                 message.RecipientsFormat(r.FormValue("format")),
		Files:                  files,
		InvalidBody:            r.FormValue("invalidBody") == "on",
		SuccessCallbackEnabled: r.FormValue("successCallbackEnabled") == "on",
		SuccessCallbackURL:     r.FormValue("successCallbackUrl"),
		FailureCallbackEnabled: r.FormValue("failureCallbackEnabled") == "on",
		FailureCallbackURL:     r.FormValue("failureCallbackUrl"),
		CallbackAuthorization:  r.FormValue("callbackAuthorization"),
	}, nil
}

func readFiles(r *http.Request) ([]message.File, error) {
	var files []message.File
	for _, header := range r.MultipartForm.File["attachments"] {
		file, err := header.Open()
		if err != nil {
			return nil, fmt.Errorf("attachment %q: %w", header.Filename, err)
		}
		content, err := io.ReadAll(file)
		file.Close()
		if err != nil {
			return nil, fmt.Errorf("attachment %q: %w", header.Filename, err)
		}
		files = append(files, message.File{Name: header.Filename, Type: header.Header.Get("Content-Type"), Content: content})
	}
	return files, nil
}
