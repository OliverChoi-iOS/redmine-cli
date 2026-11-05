package cli

import (
	"context"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"redmine-cli/internal/output"
	"redmine-cli/internal/redmine"
)

func (a *app) handleIssueAttach(ctx context.Context) output.Envelope {
	id, env := a.requireID(2, "issue id")
	if env.Error != nil {
		return env
	}
	if len(a.parsed.Positionals) != 3 || len(a.parsed.Values("file")) != 1 {
		return output.Failure("validation_failed", "One upload file is required", "Use issue attach ID --file PATH. Run once per file.", nil)
	}
	path := a.parsed.StringDefault("file", "")
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return output.Failure("file_not_found", "Upload file was not found", "Check the --file path.", map[string]any{"path": path})
	}
	if err != nil {
		return output.Failure("input_parse_error", "Could not inspect upload file", "Check the --file path and permissions.", map[string]any{"error": err.Error()})
	}
	if !info.Mode().IsRegular() || info.Size() == 0 {
		return output.Failure("validation_failed", "Invalid upload file", "Use a non-empty regular file.", nil)
	}
	file, err := os.Open(path)
	if err != nil {
		return output.Failure("input_parse_error", "Could not open upload file", "Check the --file permissions.", map[string]any{"error": err.Error()})
	}
	defer file.Close()
	filename := a.parsed.StringDefault("filename", filepath.Base(path))
	if strings.TrimSpace(filename) == "" || filename == "." || filename == ".." || strings.ContainsAny(filename, "/\\") || strings.ContainsFunc(filename, unicode.IsControl) {
		return output.Failure("validation_failed", "Invalid upload filename", "Use --filename with a name without path separators or control characters.", nil)
	}
	contentType := a.parsed.StringDefault("content-type", mime.TypeByExtension(filepath.Ext(filename)))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	if mediaType, _, err := mime.ParseMediaType(contentType); err != nil || !strings.Contains(mediaType, "/") {
		return output.Failure("validation_failed", "Invalid content type", "Use a MIME type such as application/pdf.", nil)
	}
	description := a.parsed.StringDefault("description", "")
	if a.parsed.Bool("dry-run") {
		return a.dryRun("low", "issue.attach", map[string]any{
			"file": path, "filename": filename, "filesize": info.Size(), "content_type": contentType, "description": description,
		}, map[string]any{"issue_id": id})
	}
	before, err := a.client.GetIssue(ctx, id, "attachments")
	if err != nil {
		return a.failureFromError(err)
	}
	upload, err := a.client.UploadFile(ctx, filename, file, info.Size())
	if err != nil {
		return a.failureFromError(err)
	}
	upload.ContentType = contentType
	upload.Description = description
	if err := a.client.UpdateIssue(ctx, id, map[string]any{"uploads": []redmine.Upload{upload}}); err != nil {
		env := a.failureFromError(err)
		if env.Error.Details == nil {
			env.Error.Details = map[string]any{}
		}
		env.Error.Details["upload_completed"] = true
		env.Error.Details["attachment_link_confirmed"] = false
		env.Error.Details["issue_id"] = id
		a.warnings = append(a.warnings, "file was uploaded but linking was not confirmed; check the issue before retrying, which uploads the file again")
		return env
	}
	data := map[string]any{"issue_id": id, "attached": true, "filename": filename, "before": before}
	after, err := a.client.GetIssue(ctx, id, "attachments")
	if err != nil {
		a.warnings = append(a.warnings, "file was attached but after-state fetch failed")
	} else {
		data["after"] = after
	}
	return a.success(data, nil)
}
