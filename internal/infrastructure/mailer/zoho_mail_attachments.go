package mailer

import (
	"bytes"
	"fmt"
	"net/http"
	"net/url"
)

// zohoAttachmentRef is the reference returned by the upload endpoint and sent with the message.
type zohoAttachmentRef struct {
	StoreName      string `json:"storeName"`
	AttachmentPath string `json:"attachmentPath"`
	AttachmentName string `json:"attachmentName"`
}

type zohoUploadResponse struct {
	Data zohoAttachmentRef `json:"data"`
}

// uploadAttachments uploads each attachment (RAW method) and returns the references to attach
// to the message. Zoho Mail does not accept inline attachment content in the send call.
func (s *zohoMailSender) uploadAttachments(token string, attachments []attachmentContent) ([]zohoAttachmentRef, error) {
	refs := make([]zohoAttachmentRef, 0, len(attachments))
	for _, attachment := range attachments {
		ref, err := s.uploadAttachment(token, attachment)
		if err != nil {
			return nil, fmt.Errorf("failed to upload attachment %q: %w", attachment.Name, err)
		}
		refs = append(refs, ref)
	}
	return refs, nil
}

func (s *zohoMailSender) uploadAttachment(token string, attachment attachmentContent) (zohoAttachmentRef, error) {
	endpoint := fmt.Sprintf("%s/api/accounts/%s/messages/attachments?%s",
		s.config.APIURL, url.PathEscape(s.config.AccountID), url.Values{"fileName": {attachment.Name}}.Encode())

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(attachment.Content))
	if err != nil {
		return zohoAttachmentRef{}, fmt.Errorf("failed to create upload request: %w", err)
	}
	req.Header.Set("Authorization", zohoAuthorization(token))
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Accept", "application/json")

	var result zohoUploadResponse
	if err := s.call(req, &result); err != nil {
		return zohoAttachmentRef{}, err
	}
	return result.Data, nil
}
