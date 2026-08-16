package server

import (
	"bytes"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"

	"agent-platform/backend/internal/transportmeta"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func decodeStrictJSONRequest(request *http.Request, value any) error {
	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || !strings.EqualFold(mediaType, "application/json") {
		return kratoserrors.BadRequest("application_json_required", "application/json Content-Type is required")
	}
	body, err := transportmeta.CaptureRawBody(request)
	if err != nil || len(body) == 0 || len(body) > transportmeta.MaxJSONBody {
		return kratoserrors.BadRequest("invalid_request_body", "request body is invalid")
	}
	*request = *transportmeta.WithRawBody(request, body)
	message, ok := value.(proto.Message)
	if !ok {
		return kratoserrors.BadRequest("invalid_request_body", fmt.Sprintf("unsupported request type %T", value))
	}
	if err := (protojson.UnmarshalOptions{DiscardUnknown: false}).Unmarshal(body, message); err != nil {
		return kratoserrors.BadRequest("invalid_request_body", "request body is invalid")
	}
	request.Body = io.NopCloser(bytes.NewReader(body))
	return nil
}

func encodePublicError(writer http.ResponseWriter, _ *http.Request, err error) {
	status := http.StatusInternalServerError
	code := "internal_error"
	if public := kratoserrors.FromError(err); public != nil {
		if public.Code >= 400 && public.Code <= 599 {
			status = int(public.Code)
		}
		if public.Reason != "" {
			code = public.Reason
		}
	}
	writePublicError(writer, status, code)
}
