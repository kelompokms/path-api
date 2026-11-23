package ai

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path"
	"path-api/internal/db"
	"path-api/internal/files"
	"path-api/internal/utils"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/labstack/echo/v5"
)

func (h *Handler) effectiveBaseURL() string {
	if strings.TrimSpace(h.baseURL) != "" {
		return strings.TrimRight(h.baseURL, "/")
	}
	return strings.TrimRight(utils.GetAIAPIURL(), "/")
}

func (h *Handler) effectiveAPIKey() string {
	if strings.TrimSpace(h.apiKey) != "" {
		return strings.TrimSpace(h.apiKey)
	}
	return utils.GetAIAPIKey()
}

func actorIDForClaims(claims *utils.JWTCustomClaims) string {
	if claims == nil {
		return "siswa:0"
	}
	role := strings.ToLower(strings.TrimSpace(claims.Role))
	if claims.IsAdmin() || role == "guru" {
		return fmt.Sprintf("guru:%d", claims.UserID)
	}
	return fmt.Sprintf("siswa:%d", claims.UserID)
}

func (h *Handler) logUsage(ctx context.Context, userID int32, fitur string) {
	if h.db == nil || userID <= 0 {
		return
	}
	if err := h.db.CreateAiUsageLog(ctx, db.CreateAiUsageLogParams{
		PenggunaID: userID,
		Fitur:      fitur,
	}); err != nil {
		log.Printf("warning: failed to log AI usage (%s, user %d): %v", fitur, userID, err)
	}
}

func extractRemoteAIError(body []byte, statusCode int) string {
	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err == nil {
		if errObj, ok := parsed["error"].(map[string]any); ok {
			if msg, ok := errObj["message"].(string); ok && strings.TrimSpace(msg) != "" {
				return strings.TrimSpace(msg)
			}
		}
		if detail, ok := parsed["detail"].(string); ok && strings.TrimSpace(detail) != "" {
			return strings.TrimSpace(detail)
		}
		if detailList, ok := parsed["detail"].([]any); ok && len(detailList) > 0 {
			if first, ok := detailList[0].(map[string]any); ok {
				if msg, ok := first["msg"].(string); ok && strings.TrimSpace(msg) != "" {
					return strings.TrimSpace(msg)
				}
			}
		}
		if msg, ok := parsed["message"].(string); ok && strings.TrimSpace(msg) != "" {
			return strings.TrimSpace(msg)
		}
	}
	return fmt.Sprintf("Layanan AI mengembalikan status %d", statusCode)
}

func writeMultipartFile(w *multipart.Writer, fieldName, filename, contentType string, data []byte) error {
	h := make(textproto.MIMEHeader)
	safeName := strings.NewReplacer(`"`, "", "\r", "", "\n", "").Replace(filename)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, fieldName, safeName))
	if contentType != "" {
		h.Set("Content-Type", contentType)
	} else {
		h.Set("Content-Type", "application/octet-stream")
	}
	part, err := w.CreatePart(h)
	if err != nil {
		return err
	}
	_, err = part.Write(data)
	return err
}

func mimeForFilename(name string) (string, string) {
	ext := strings.ToLower(strings.TrimPrefix(path.Ext(name), "."))
	switch ext {
	case "pdf":
		return name, "application/pdf"
	case "docx":
		return name, "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case "txt":
		return name, "text/plain; charset=utf-8"
	case "csv":
		base := strings.TrimSuffix(name, path.Ext(name))
		return base + ".txt", "text/plain; charset=utf-8"
	default:
		return name, "application/octet-stream"
	}
}

type remoteArtifactRef struct {
	Name         string `json:"name"`
	MediaType    string `json:"media_type"`
	Availability string `json:"availability"`
	SizeBytes    int64  `json:"size_bytes"`
}

type remoteJobEnvelope struct {
	JobID    string          `json:"job_id"`
	Status   string          `json:"status"`
	Message  string          `json:"message"`
	Error    any             `json:"error"`
	Result   json.RawMessage `json:"result"`
	Artifact *struct {
		Name         string             `json:"name"`
		MediaType    string             `json:"media_type"`
		Availability string             `json:"availability"`
		SizeBytes    int64              `json:"size_bytes"`
		JSON         *remoteArtifactRef `json:"json"`
	} `json:"artifact"`
}

func (h *Handler) pollJob(ctx context.Context, jobID, actorID string) (*remoteJobEnvelope, error) {
	url := fmt.Sprintf("%s/v1/jobs/%s", h.effectiveBaseURL(), jobID)
	interval := h.pollInterval
	if interval <= 0 {
		interval = 1500 * time.Millisecond
	}
	maxPolls := h.maxPolls
	if maxPolls <= 0 {
		maxPolls = 80
	}

	for attempt := 0; attempt < maxPolls; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(interval):
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("X-API-Key", h.effectiveAPIKey())
		req.Header.Set("X-Actor-Id", actorID)

		resp, err := h.httpClient.Do(req)
		if err != nil {
			return nil, err
		}
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, errors.New(extractRemoteAIError(body, resp.StatusCode))
		}

		var env remoteJobEnvelope
		if err := json.Unmarshal(body, &env); err != nil {
			return nil, fmt.Errorf("gagal membaca status job AI: %w", err)
		}

		switch strings.ToLower(strings.TrimSpace(env.Status)) {
		case "done", "completed", "succeeded":
			return &env, nil
		case "failed", "error", "cancelled":
			msg := strings.TrimSpace(env.Message)
			if errMap, ok := env.Error.(map[string]any); ok {
				if m, ok := errMap["message"].(string); ok && strings.TrimSpace(m) != "" {
					msg = strings.TrimSpace(m)
				}
			} else if errStr, ok := env.Error.(string); ok && strings.TrimSpace(errStr) != "" {
				msg = strings.TrimSpace(errStr)
			}
			if msg == "" {
				msg = "Pemrosesan AI gagal"
			}
			return nil, errors.New(msg)
		}
	}

	return nil, errors.New("Waktu pemrosesan AI habis, silakan coba lagi")
}

func (h *Handler) downloadArtifact(ctx context.Context, availability, actorID string) ([]byte, error) {
	targetURL := availability
	if !strings.HasPrefix(targetURL, "http://") && !strings.HasPrefix(targetURL, "https://") {
		if !strings.HasPrefix(targetURL, "/") {
			targetURL = "/" + targetURL
		}
		targetURL = h.effectiveBaseURL() + targetURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-API-Key", h.effectiveAPIKey())
	req.Header.Set("X-Actor-Id", actorID)

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, errors.New(extractRemoteAIError(body, resp.StatusCode))
	}
	return body, nil
}

func extractDocxText(data []byte) string {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return ""
	}
	for _, f := range zr.File {
		if f.Name != "word/document.xml" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return ""
		}
		defer rc.Close()
		dec := xml.NewDecoder(io.LimitReader(rc, 2<<20))
		var sb strings.Builder
		for {
			tok, err := dec.Token()
			if err != nil {
				break
			}
			switch t := tok.(type) {
			case xml.CharData:
				sb.Write(t)
			case xml.EndElement:
				if t.Name.Local == "p" || t.Name.Local == "tr" {
					sb.WriteByte('\n')
				} else if t.Name.Local == "tc" {
					sb.WriteByte('\t')
				}
			}
		}
		return strings.TrimSpace(sb.String())
	}
	return ""
}

func extractUploadText(paths []string) string {
	parts := make([]string, 0, len(paths))
	for _, rawPath := range paths {
		name, data, ok := readSafeUploadFile(rawPath)
		if !ok {
			continue
		}
		ext := strings.ToLower(strings.TrimPrefix(path.Ext(name), "."))
		switch ext {
		case "txt", "csv":
			if utf8.Valid(data) {
				if t := strings.TrimSpace(string(data)); t != "" {
					parts = append(parts, t)
				}
			}
		case "docx":
			if t := extractDocxText(data); t != "" {
				parts = append(parts, t)
			}
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n\n"))
}

func isFreeModelRateLimitFailure(statusCode int, body []byte) bool {
	if statusCode == http.StatusTooManyRequests || statusCode == http.StatusBadGateway || statusCode == http.StatusServiceUnavailable {
		lower := strings.ToLower(string(body))
		return strings.Contains(lower, "llm_unavailable") ||
			strings.Contains(lower, "rate-limited") ||
			strings.Contains(lower, "429") ||
			strings.Contains(lower, "tidak merespons")
	}
	return false
}

// Chat godoc
//
//	@Summary		Chat Bantuan Belajar dengan AI
//	@Description	Mengirimkan pertanyaan materi ke layanan AI (ai.pathclass.app /v1/chat) dengan dukungan riwayat percakapan
//	@Tags			AI
//	@Accept			json
//	@Produce		json
//	@Param			request				body		ChatRequest	true	"Permintaan Chat AI"
//	@Success		200					{object}	ChatResponse
//	@Failure		400,401,502,503		{object}	utils.HTTPResponse
//	@Router			/ai/chat [post]
func (h *Handler) Chat(c *echo.Context) error {
	claims, err := utils.GetClaims(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "Tidak dapat mengakses data klaim")
	}

	apiKey := h.effectiveAPIKey()
	if apiKey == "" {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "Layanan AI belum dikonfigurasi di server")
	}

	req := new(ChatRequest)
	if err := c.Bind(req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	req.Pesan = strings.TrimSpace(req.Pesan)
	if err := c.Validate(req); err != nil {
		return utils.ValidationErrorHandler(c, err)
	}

	pesan := req.Pesan
	activeModes := make([]string, 0, len(req.Modes))
	for _, m := range req.Modes {
		m = strings.TrimSpace(m)
		if m != "" && !strings.EqualFold(m, "Kombinasi") {
			activeModes = append(activeModes, m)
		}
	}
	if len(activeModes) > 0 {
		pesan = fmt.Sprintf("Tolong jelaskan dengan bantuan format %s.\n\nPertanyaan: %s", strings.Join(activeModes, ", "), pesan)
	}

	isiMateri := strings.TrimSpace(req.IsiMateri)
	extractedDocText := extractUploadText(req.LampiranPaths)
	if extractedDocText != "" {
		if isiMateri != "" {
			isiMateri = isiMateri + "\n\n" + extractedDocText
		} else {
			isiMateri = extractedDocText
		}
	}

	// When isi_materi is only a short post title/brief description (without a full document),
	// ai.pathclass.app /v1/chat treats isi_materi as a strict closed-book text and refuses
	// general questions about the topic. Embed short topic context into pesan instead.
	upstreamIsiMateri := isiMateri
	if upstreamIsiMateri != "" && extractedDocText == "" && utf8.RuneCountInString(upstreamIsiMateri) < 400 {
		pesan = fmt.Sprintf("Konteks topik materi pembelajaran: %s\n\n%s", upstreamIsiMateri, pesan)
		upstreamIsiMateri = ""
	}

	riwayat := req.Riwayat
	if riwayat == nil {
		riwayat = []ChatMessage{}
	}

	doUpstreamChat := func(useFree bool) (*http.Response, []byte, error) {
		upstreamPayload := map[string]any{
			"pesan":                pesan,
			"riwayat":              riwayat,
			"isi_materi":           upstreamIsiMateri,
			"jenjang":              strings.TrimSpace(req.Jenjang),
			"gunakan_model_gratis": useFree,
		}
		payloadBytes, err := json.Marshal(upstreamPayload)
		if err != nil {
			return nil, nil, err
		}
		httpReq, err := http.NewRequestWithContext(
			c.Request().Context(),
			http.MethodPost,
			h.effectiveBaseURL()+"/v1/chat",
			bytes.NewReader(payloadBytes),
		)
		if err != nil {
			return nil, nil, err
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("X-API-Key", apiKey)
		httpReq.Header.Set("X-Actor-Id", actorIDForClaims(claims))

		resp, err := h.httpClient.Do(httpReq)
		if err != nil {
			return nil, nil, err
		}
		defer resp.Body.Close()
		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return resp, nil, readErr
		}
		return resp, body, nil
	}

	resp, body, err := doUpstreamChat(req.GunakanModelGratis)
	if err != nil {
		log.Println("ai chat request error:", err)
		return echo.NewHTTPError(http.StatusBadGateway, "Tidak dapat terhubung ke server AI")
	}
	if req.GunakanModelGratis && isFreeModelRateLimitFailure(resp.StatusCode, body) {
		if retryResp, retryBody, retryErr := doUpstreamChat(false); retryErr == nil {
			resp = retryResp
			body = retryBody
		}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return echo.NewHTTPError(http.StatusBadGateway, extractRemoteAIError(body, resp.StatusCode))
	}

	var result ChatResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return echo.NewHTTPError(http.StatusBadGateway, "Respons server AI tidak valid")
	}

	h.logUsage(c.Request().Context(), claims.UserID, "bantuan_ai")
	return c.JSON(http.StatusOK, result)
}

// ExplainMateri godoc
//
//	@Summary		Penjelasan Visual Materi dengan AI
//	@Description	Membuat penjelasan materi dalam bentuk dokumen PDF visual (tabel, diagram, ilustrasi, atau kombinasi) melalui ai.pathclass.app /v1/materials/explain
//	@Tags			AI
//	@Accept			json
//	@Produce		json
//	@Param			request				body		ExplainMateriRequest	true	"Permintaan Penjelasan Materi AI"
//	@Success		200					{object}	ExplainMateriResponse
//	@Failure		400,401,500,502,503	{object}	utils.HTTPResponse
//	@Router			/ai/materi/explain [post]
func (h *Handler) ExplainMateri(c *echo.Context) error {
	claims, err := utils.GetClaims(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "Tidak dapat mengakses data klaim")
	}

	apiKey := h.effectiveAPIKey()
	if apiKey == "" {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "Layanan AI belum dikonfigurasi di server")
	}

	req := new(ExplainMateriRequest)
	if err := c.Bind(req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	req.Pertanyaan = strings.TrimSpace(req.Pertanyaan)
	if err := c.Validate(req); err != nil {
		return utils.ValidationErrorHandler(c, err)
	}

	// Image generation endpoints on ai.pathclass.app cannot use the free model tier.
	gunakanGratis := req.GunakanModelGratis && !req.Ilustrasi && !req.Kombinasi

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("pertanyaan", req.Pertanyaan)
	_ = mw.WriteField("deskripsi_materi", strings.TrimSpace(req.DeskripsiMateri))
	_ = mw.WriteField("isi_materi", strings.TrimSpace(req.IsiMateri))
	_ = mw.WriteField("jenjang", strings.TrimSpace(req.Jenjang))
	_ = mw.WriteField("tabel", strconv.FormatBool(req.Tabel))
	_ = mw.WriteField("diagram", strconv.FormatBool(req.Diagram))
	_ = mw.WriteField("ilustrasi", strconv.FormatBool(req.Ilustrasi))
	_ = mw.WriteField("kombinasi", strconv.FormatBool(req.Kombinasi))
	_ = mw.WriteField("gunakan_model_gratis", strconv.FormatBool(gunakanGratis))

	// Attach the first readable PDF/DOCX/TXT uploaded material file if available.
	for _, rawPath := range req.LampiranPaths {
		if name, data, ok := readSafeUploadFile(rawPath); ok && isPreferredAnswerDoc(name) {
			upName, upMime := mimeForFilename(name)
			_ = writeMultipartFile(mw, "berkas_materi", upName, upMime, data)
			break
		}
	}
	if err := mw.Close(); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal menyiapkan permintaan penjelasan AI")
	}

	actorID := actorIDForClaims(claims)
	httpReq, err := http.NewRequestWithContext(
		c.Request().Context(),
		http.MethodPost,
		h.effectiveBaseURL()+"/v1/materials/explain",
		&buf,
	)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal membuat permintaan AI")
	}
	httpReq.Header.Set("Content-Type", mw.FormDataContentType())
	httpReq.Header.Set("X-API-Key", apiKey)
	httpReq.Header.Set("X-Actor-Id", actorID)

	resp, err := h.httpClient.Do(httpReq)
	if err != nil {
		log.Println("ai explain materi error:", err)
		return echo.NewHTTPError(http.StatusBadGateway, "Tidak dapat terhubung ke server AI")
	}
	body, readErr := io.ReadAll(resp.Body)
	resp.Body.Close()
	if readErr != nil {
		return echo.NewHTTPError(http.StatusBadGateway, "Gagal membaca respons server AI")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return echo.NewHTTPError(http.StatusBadGateway, extractRemoteAIError(body, resp.StatusCode))
	}

	var created remoteJobEnvelope
	if err := json.Unmarshal(body, &created); err != nil || strings.TrimSpace(created.JobID) == "" {
		return echo.NewHTTPError(http.StatusBadGateway, "Job ID penjelasan AI tidak valid")
	}

	finalJob, err := h.pollJob(c.Request().Context(), created.JobID, actorID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadGateway, err.Error())
	}

	var explainResult struct {
		Judul            string `json:"judul"`
		DeskripsiSingkat string `json:"deskripsi_singkat"`
	}
	_ = json.Unmarshal(finalJob.Result, &explainResult)

	judul := strings.TrimSpace(explainResult.Judul)
	jawaban := strings.TrimSpace(explainResult.DeskripsiSingkat)
	if jawaban == "" {
		jawaban = "Penjelasan visual AI telah selesai dibuat. Buka dokumen PDF di bawah ini untuk melihat tabel, diagram, atau ilustrasi lengkapnya."
	}

	var savedFile *files.UploadedPostinganFile
	if finalJob.Artifact != nil && strings.TrimSpace(finalJob.Artifact.Availability) != "" {
		artifactBytes, err := h.downloadArtifact(c.Request().Context(), finalJob.Artifact.Availability, actorID)
		if err != nil {
			log.Println("ai download explain artifact error:", err)
			return echo.NewHTTPError(http.StatusBadGateway, "Gagal mengunduh file penjelasan AI")
		}
		fileName := strings.TrimSpace(finalJob.Artifact.Name)
		if fileName == "" {
			fileName = "penjelasan-ai.pdf"
		}
		saved, err := files.SaveUploadedBytes(fileName, path.Ext(fileName), artifactBytes)
		if err != nil {
			log.Println("ai save explain artifact error:", err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Gagal menyimpan file penjelasan AI")
		}
		savedFile = &saved
	}

	h.logUsage(c.Request().Context(), claims.UserID, "bantuan_ai")
	return c.JSON(http.StatusOK, ExplainMateriResponse{
		Judul:   judul,
		Jawaban: jawaban,
		File:    savedFile,
	})
}

// GenerateMateri godoc
//
//	@Summary		Buat Materi Pembelajaran dengan AI (Guru)
//	@Description	Membuat materi pembelajaran berupa dokumen PDF atau PowerPoint (PPTX) melalui ai.pathclass.app /v1/materials/generate (khusus guru atau admin)
//	@Tags			AI
//	@Accept			json
//	@Produce		json
//	@Param			request					body		GenerateMateriRequest	true	"Permintaan Pembuatan Materi AI"
//	@Success		200						{object}	GenerateMateriResponse
//	@Failure		400,401,403,500,502,503	{object}	utils.HTTPResponse
//	@Router			/ai/materi/generate [post]
func (h *Handler) GenerateMateri(c *echo.Context) error {
	claims, err := utils.GetClaims(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "Tidak dapat mengakses data klaim")
	}
	if !claims.IsAdmin() && !strings.EqualFold(claims.Role, "guru") {
		return echo.NewHTTPError(http.StatusForbidden, "Hanya pengajar yang dapat membuat materi dengan AI")
	}

	apiKey := h.effectiveAPIKey()
	if apiKey == "" {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "Layanan AI belum dikonfigurasi di server")
	}

	req := new(GenerateMateriRequest)
	if err := c.Bind(req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	req.Judul = strings.TrimSpace(req.Judul)
	if err := c.Validate(req); err != nil {
		return utils.ValidationErrorHandler(c, err)
	}

	// Image generation endpoints on ai.pathclass.app cannot use the free model tier.
	gunakanGratis := req.GunakanModelGratis && !req.Ilustrasi && !req.Kombinasi
	formatKeluaran := strings.ToLower(strings.TrimSpace(req.FormatKeluaran))
	if formatKeluaran != "pptx" {
		formatKeluaran = "pdf"
	}

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("judul", req.Judul)
	_ = mw.WriteField("deskripsi_materi", strings.TrimSpace(req.DeskripsiMateri))
	_ = mw.WriteField("jenjang", strings.TrimSpace(req.Jenjang))
	_ = mw.WriteField("tabel", strconv.FormatBool(req.Tabel))
	_ = mw.WriteField("diagram", strconv.FormatBool(req.Diagram))
	_ = mw.WriteField("ilustrasi", strconv.FormatBool(req.Ilustrasi))
	_ = mw.WriteField("kombinasi", strconv.FormatBool(req.Kombinasi))
	_ = mw.WriteField("format_keluaran", formatKeluaran)
	_ = mw.WriteField("gunakan_model_gratis", strconv.FormatBool(gunakanGratis))

	for _, rawPath := range req.LampiranPaths {
		if name, data, ok := readSafeUploadFile(rawPath); ok && isPreferredAnswerDoc(name) {
			upName, upMime := mimeForFilename(name)
			_ = writeMultipartFile(mw, "berkas_materi", upName, upMime, data)
			break
		}
	}
	if err := mw.Close(); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal menyiapkan data materi AI")
	}

	actorID := actorIDForClaims(claims)
	httpReq, err := http.NewRequestWithContext(
		c.Request().Context(),
		http.MethodPost,
		h.effectiveBaseURL()+"/v1/materials/generate",
		&buf,
	)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal membuat permintaan AI")
	}
	httpReq.Header.Set("Content-Type", mw.FormDataContentType())
	httpReq.Header.Set("X-API-Key", apiKey)
	httpReq.Header.Set("X-Actor-Id", actorID)

	resp, err := h.httpClient.Do(httpReq)
	if err != nil {
		log.Println("ai generate materi error:", err)
		return echo.NewHTTPError(http.StatusBadGateway, "Tidak dapat terhubung ke server AI")
	}
	body, readErr := io.ReadAll(resp.Body)
	resp.Body.Close()
	if readErr != nil {
		return echo.NewHTTPError(http.StatusBadGateway, "Gagal membaca respons server AI")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return echo.NewHTTPError(http.StatusBadGateway, extractRemoteAIError(body, resp.StatusCode))
	}

	var created remoteJobEnvelope
	if err := json.Unmarshal(body, &created); err != nil || strings.TrimSpace(created.JobID) == "" {
		return echo.NewHTTPError(http.StatusBadGateway, "Job ID AI tidak valid")
	}

	finalJob, err := h.pollJob(c.Request().Context(), created.JobID, actorID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadGateway, err.Error())
	}

	var genResult struct {
		Judul            string `json:"judul"`
		DeskripsiSingkat string `json:"deskripsi_singkat"`
	}
	_ = json.Unmarshal(finalJob.Result, &genResult)

	judul := strings.TrimSpace(genResult.Judul)
	if judul == "" {
		judul = req.Judul
	}
	deskripsi := strings.TrimSpace(genResult.DeskripsiSingkat)
	if deskripsi == "" {
		deskripsi = strings.TrimSpace(req.DeskripsiMateri)
	}

	lampiran := make([]files.UploadedPostinganFile, 0, 1)
	if finalJob.Artifact != nil && strings.TrimSpace(finalJob.Artifact.Availability) != "" {
		artifactBytes, err := h.downloadArtifact(c.Request().Context(), finalJob.Artifact.Availability, actorID)
		if err != nil {
			log.Println("ai download materi artifact error:", err)
			return echo.NewHTTPError(http.StatusBadGateway, "Gagal mengunduh file materi hasil AI")
		}
		fileName := strings.TrimSpace(finalJob.Artifact.Name)
		if fileName == "" {
			fileName = "materi-ai.pdf"
		}
		saved, err := files.SaveUploadedBytes(fileName, path.Ext(fileName), artifactBytes)
		if err != nil {
			log.Println("ai save materi artifact error:", err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Gagal menyimpan file materi hasil AI")
		}
		lampiran = append(lampiran, saved)
	}

	h.logUsage(c.Request().Context(), claims.UserID, "buat_materi")
	return c.JSON(http.StatusOK, GenerateMateriResponse{
		Judul:     judul,
		Deskripsi: deskripsi,
		Lampiran:  lampiran,
	})
}

func readSafeUploadFile(rawPath string) (string, []byte, bool) {
	trimmed := strings.TrimSpace(rawPath)
	if trimmed == "" || strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
		return "", nil, false
	}
	rel := strings.TrimPrefix(trimmed, "/")
	rel = strings.TrimPrefix(rel, "uploads/")
	if rel == "" || strings.Contains(rel, "..") {
		return "", nil, false
	}
	full := filepath.Join(files.UploadRoot, filepath.FromSlash(rel))
	absRoot, err1 := filepath.Abs(files.UploadRoot)
	absFull, err2 := filepath.Abs(full)
	if err1 != nil || err2 != nil || !strings.HasPrefix(absFull, absRoot) {
		return "", nil, false
	}
	data, err := os.ReadFile(absFull)
	if err != nil || len(data) == 0 {
		return "", nil, false
	}
	return path.Base(trimmed), data, true
}

func isPreferredAnswerDoc(name string) bool {
	switch strings.ToLower(strings.TrimPrefix(path.Ext(name), ".")) {
	case "pdf", "docx", "txt", "csv":
		return true
	default:
		return false
	}
}

// KoreksiTugas godoc
//
//	@Summary		Koreksi Jawaban Tugas Siswa dengan AI (Guru)
//	@Description	Mengoreksi kiriman tugas siswa menggunakan kunci jawaban (file PDF/DOCX/TXT/CSV atau deskripsi kunci) melalui ai.pathclass.app /v1/assessments/grade
//	@Tags			AI
//	@Accept			multipart/form-data
//	@Produce		json
//	@Param			kode_kelas				formData	string	true	"Kode Kelas"
//	@Param			postingan_id			formData	int		true	"ID Postingan Tugas"
//	@Param			pengguna_ids			formData	string	false	"Daftar ID pengguna siswa dipisahkan koma"
//	@Param			deskripsi_kunci			formData	string	false	"Deskripsi kunci jawaban teks"
//	@Param			berkas_kunci			formData	file	false	"Berkas kunci jawaban (PDF, DOCX, TXT, CSV)"
//	@Success		200						{object}	KoreksiTugasResponse
//	@Failure		400,401,403,404,500,502,503	{object}	utils.HTTPResponse
//	@Router			/ai/koreksi [post]
func (h *Handler) KoreksiTugas(c *echo.Context) error {
	claims, err := utils.GetClaims(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "Tidak dapat mengakses data klaim")
	}

	apiKey := h.effectiveAPIKey()
	if apiKey == "" {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "Layanan AI belum dikonfigurasi di server")
	}

	if err := c.Request().ParseMultipartForm(16 << 20); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Format form koreksi AI tidak valid")
	}
	if form := c.Request().MultipartForm; form != nil {
		defer form.RemoveAll()
	}

	kodeKelas := strings.TrimSpace(c.FormValue("kode_kelas"))
	postinganIDRaw := strings.TrimSpace(c.FormValue("postingan_id"))
	deskripsiKunci := strings.TrimSpace(c.FormValue("deskripsi_kunci"))
	penggunaIDsRaw := strings.TrimSpace(c.FormValue("pengguna_ids"))

	if kodeKelas == "" || postinganIDRaw == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "kode_kelas dan postingan_id wajib diisi")
	}
	postinganID64, err := strconv.ParseInt(postinganIDRaw, 10, 32)
	if err != nil || postinganID64 <= 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "postingan_id tidak valid")
	}
	postinganID := int32(postinganID64)

	ctx := c.Request().Context()
	kelasRow, err := h.db.GetKelas(ctx, kodeKelas)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "Kelas tidak ditemukan")
	}
	if !claims.IsAdmin() {
		isGuru, err := h.db.IsGuruKelas(ctx, db.IsGuruKelasParams{
			PenggunaID: claims.UserID,
			KelasID:    kelasRow.ID,
		})
		if err != nil || !isGuru {
			return echo.NewHTTPError(http.StatusForbidden, "Hanya pengajar kelas yang dapat menggunakan koreksi AI")
		}
	}

	postRow, err := h.db.GetPostinganByID(ctx, db.GetPostinganByIDParams{
		ID:      postinganID,
		KelasID: kelasRow.ID,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "Tugas tidak ditemukan")
	}

	pengajarNama := "Pengajar"
	if userRow, err := h.db.GetPengguna(ctx, claims.UserID); err == nil && strings.TrimSpace(userRow.Nama) != "" {
		pengajarNama = strings.TrimSpace(userRow.Nama)
	}

	submissions, err := h.db.ListPengumpulanTugas(ctx, postinganID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal memuat daftar pengumpulan tugas")
	}

	selectedIDs := make(map[int32]bool)
	if penggunaIDsRaw != "" {
		for _, tok := range strings.Split(penggunaIDsRaw, ",") {
			if id, err := strconv.ParseInt(strings.TrimSpace(tok), 10, 32); err == nil && id > 0 {
				selectedIDs[int32(id)] = true
			}
		}
	}

	targetSubs := make([]db.ListPengumpulanTugasRow, 0, len(submissions))
	for _, sub := range submissions {
		if len(selectedIDs) > 0 && !selectedIDs[sub.PenggunaID] {
			continue
		}
		targetSubs = append(targetSubs, sub)
	}
	if len(targetSubs) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Belum ada kiriman siswa yang dipilih untuk dikoreksi")
	}

	// Read answer key file (optional if deskripsi_kunci is supplied).
	var keyFileName string
	var keyFileBytes []byte
	if fh, err := c.FormFile("berkas_kunci"); err == nil && fh != nil && fh.Size > 0 {
		if fh.Size > files.MaxPostinganFileSize {
			return echo.NewHTTPError(http.StatusBadRequest, "Ukuran file kunci jawaban terlalu besar (maks. 10 MB)")
		}
		f, openErr := fh.Open()
		if openErr != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "Gagal membaca file kunci jawaban")
		}
		data, readErr := io.ReadAll(f)
		f.Close()
		if readErr != nil || len(data) == 0 {
			return echo.NewHTTPError(http.StatusBadRequest, "File kunci jawaban kosong")
		}
		keyFileName = fh.Filename
		keyFileBytes = data
	} else if deskripsiKunci != "" {
		keyFileName = "kunci_jawaban.txt"
		keyFileBytes = []byte(fmt.Sprintf("Kunci Jawaban Tugas: %s\n\n%s\n", postRow.Judul, deskripsiKunci))
	} else {
		return echo.NewHTTPError(http.StatusBadRequest, "Unggah file kunci jawaban atau isi deskripsi kunci jawaban")
	}

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	mapel := strings.TrimSpace(kelasRow.Nama)
	_ = mw.WriteField("pengajar", pengajarNama)
	_ = mw.WriteField("mata_pelajaran", mapel)
	_ = mw.WriteField("kelas", strings.TrimSpace(kelasRow.Nama))
	_ = mw.WriteField("judul_tugas", strings.TrimSpace(postRow.Judul))
	_ = mw.WriteField("on_low_confidence", "continue")

	upKeyName, upKeyMime := mimeForFilename(keyFileName)
	if err := writeMultipartFile(mw, "berkas_kunci", upKeyName, upKeyMime, keyFileBytes); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal menyiapkan file kunci jawaban")
	}

	subByEmail := make(map[string]db.ListPengumpulanTugasRow, len(targetSubs))
	subByName := make(map[string]db.ListPengumpulanTugasRow, len(targetSubs))
	for _, sub := range targetSubs {
		email := strings.TrimSpace(sub.Email)
		if email == "" {
			email = fmt.Sprintf("siswa%d@path.local", sub.PenggunaID)
		}
		nama := strings.TrimSpace(sub.Nama)
		if nama == "" {
			nama = fmt.Sprintf("Siswa %d", sub.PenggunaID)
		}
		subByEmail[strings.ToLower(email)] = sub
		subByName[strings.ToLower(nama)] = sub

		// Prefer an uploaded document file (.pdf/.docx/.txt/.csv); otherwise fall back to a .txt summary.
		var chosenName string
		var chosenBytes []byte
		for _, lamp := range sub.Lampiran {
			if name, data, ok := readSafeUploadFile(lamp); ok && isPreferredAnswerDoc(name) {
				chosenName = name
				chosenBytes = data
				break
			}
		}
		if len(chosenBytes) == 0 {
			summary := fmt.Sprintf("Kiriman siswa: %s (%s)\nLampiran: %s\n", nama, email, strings.Join(sub.Lampiran, ", "))
			chosenName = fmt.Sprintf("jawaban_%d.txt", sub.PenggunaID)
			chosenBytes = []byte(summary)
		}

		upName, upMime := mimeForFilename(chosenName)
		if err := writeMultipartFile(mw, "jawaban_siswa", upName, upMime, chosenBytes); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Gagal menyiapkan file jawaban siswa")
		}
		_ = mw.WriteField("email", email)
		_ = mw.WriteField("nama", nama)
	}
	if err := mw.Close(); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal menyiapkan permintaan koreksi AI")
	}

	actorID := actorIDForClaims(claims)
	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		h.effectiveBaseURL()+"/v1/assessments/grade",
		&buf,
	)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal membuat permintaan koreksi AI")
	}
	httpReq.Header.Set("Content-Type", mw.FormDataContentType())
	httpReq.Header.Set("X-API-Key", apiKey)
	httpReq.Header.Set("X-Actor-Id", actorID)

	resp, err := h.httpClient.Do(httpReq)
	if err != nil {
		log.Println("ai grade request error:", err)
		return echo.NewHTTPError(http.StatusBadGateway, "Tidak dapat terhubung ke server AI")
	}
	body, readErr := io.ReadAll(resp.Body)
	resp.Body.Close()
	if readErr != nil {
		return echo.NewHTTPError(http.StatusBadGateway, "Gagal membaca respons server AI")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return echo.NewHTTPError(http.StatusBadGateway, extractRemoteAIError(body, resp.StatusCode))
	}

	var created remoteJobEnvelope
	if err := json.Unmarshal(body, &created); err != nil || strings.TrimSpace(created.JobID) == "" {
		return echo.NewHTTPError(http.StatusBadGateway, "Job ID koreksi AI tidak valid")
	}

	finalJob, err := h.pollJob(ctx, created.JobID, actorID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadGateway, err.Error())
	}

	var gradeResult struct {
		Analisa []string `json:"deskripsi_hasil_analisa"`
		Siswa   []struct {
			Email string  `json:"email"`
			Nama  string  `json:"nama"`
			Nilai float64 `json:"nilai"`
			Label string  `json:"label"`
			Saran string  `json:"saran"`
		} `json:"siswa"`
	}
	_ = json.Unmarshal(finalJob.Result, &gradeResult)

	hasil := make([]KoreksiSiswaResult, 0, len(gradeResult.Siswa))
	for _, s := range gradeResult.Siswa {
		var matched db.ListPengumpulanTugasRow
		if m, ok := subByEmail[strings.ToLower(strings.TrimSpace(s.Email))]; ok {
			matched = m
		} else if m, ok := subByName[strings.ToLower(strings.TrimSpace(s.Nama))]; ok {
			matched = m
		}
		score := int(math.Round(s.Nilai))
		if score < 0 {
			score = 0
		} else if score > 100 {
			score = 100
		}
		hasil = append(hasil, KoreksiSiswaResult{
			PenggunaID: matched.PenggunaID,
			Email:      s.Email,
			Nama:       s.Nama,
			Nilai:      score,
			Label:      s.Label,
			Saran:      s.Saran,
		})
	}

	var rekapFile *files.UploadedPostinganFile
	if finalJob.Artifact != nil && strings.TrimSpace(finalJob.Artifact.Availability) != "" {
		if artifactBytes, err := h.downloadArtifact(ctx, finalJob.Artifact.Availability, actorID); err == nil {
			fileName := strings.TrimSpace(finalJob.Artifact.Name)
			if fileName == "" {
				fileName = "rekap-koreksi-ai.xlsx"
			}
			if saved, err := files.SaveUploadedBytes(fileName, path.Ext(fileName), artifactBytes); err == nil {
				rekapFile = &saved
			}
		}
	}

	analisa := gradeResult.Analisa
	if analisa == nil {
		analisa = []string{}
	}

	h.logUsage(ctx, claims.UserID, "koreksi_ai")
	return c.JSON(http.StatusOK, KoreksiTugasResponse{
		RekapFile: rekapFile,
		Analisa:   analisa,
		Hasil:     hasil,
	})
}

type assessmentContext struct {
	kelasRow     db.Kela
	postRow      db.Postingan
	pengajarNama string
	targetSubs   []db.ListPengumpulanTugasRow
}

func (h *Handler) loadAssessmentContext(
	ctx context.Context,
	claims *utils.JWTCustomClaims,
	kodeKelas string,
	postinganID int32,
	selectedIDs map[int32]bool,
	minSubs int,
	minSubsErr string,
) (*assessmentContext, error) {
	kelasRow, err := h.db.GetKelas(ctx, kodeKelas)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusNotFound, "Kelas tidak ditemukan")
	}
	if !claims.IsAdmin() {
		isGuru, err := h.db.IsGuruKelas(ctx, db.IsGuruKelasParams{
			PenggunaID: claims.UserID,
			KelasID:    kelasRow.ID,
		})
		if err != nil || !isGuru {
			return nil, echo.NewHTTPError(http.StatusForbidden, "Hanya pengajar kelas yang dapat menggunakan fitur ini")
		}
	}

	postRow, err := h.db.GetPostinganByID(ctx, db.GetPostinganByIDParams{
		ID:      postinganID,
		KelasID: kelasRow.ID,
	})
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusNotFound, "Tugas tidak ditemukan")
	}

	pengajarNama := "Pengajar"
	if userRow, err := h.db.GetPengguna(ctx, claims.UserID); err == nil && strings.TrimSpace(userRow.Nama) != "" {
		pengajarNama = strings.TrimSpace(userRow.Nama)
	}

	submissions, err := h.db.ListPengumpulanTugas(ctx, postinganID)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Gagal memuat daftar pengumpulan tugas")
	}

	targetSubs := make([]db.ListPengumpulanTugasRow, 0, len(submissions))
	for _, sub := range submissions {
		if len(selectedIDs) > 0 && !selectedIDs[sub.PenggunaID] {
			continue
		}
		targetSubs = append(targetSubs, sub)
	}
	if len(targetSubs) < minSubs {
		return nil, echo.NewHTTPError(http.StatusBadRequest, minSubsErr)
	}

	return &assessmentContext{
		kelasRow:     kelasRow,
		postRow:      postRow,
		pengajarNama: pengajarNama,
		targetSubs:   targetSubs,
	}, nil
}

func appendSubmissionsMultipart(mw *multipart.Writer, targetSubs []db.ListPengumpulanTugasRow) error {
	for _, sub := range targetSubs {
		email := strings.TrimSpace(sub.Email)
		if email == "" {
			email = fmt.Sprintf("siswa%d@path.local", sub.PenggunaID)
		}
		nama := strings.TrimSpace(sub.Nama)
		if nama == "" {
			nama = fmt.Sprintf("Siswa %d", sub.PenggunaID)
		}
		var chosenName string
		var chosenBytes []byte
		for _, lamp := range sub.Lampiran {
			if name, data, ok := readSafeUploadFile(lamp); ok && isPreferredAnswerDoc(name) {
				chosenName = name
				chosenBytes = data
				break
			}
		}
		if len(chosenBytes) == 0 {
			summary := fmt.Sprintf("Kiriman siswa: %s (%s)\nLampiran: %s\n", nama, email, strings.Join(sub.Lampiran, ", "))
			chosenName = fmt.Sprintf("jawaban_%d.txt", sub.PenggunaID)
			chosenBytes = []byte(summary)
		}
		upName, upMime := mimeForFilename(chosenName)
		if err := writeMultipartFile(mw, "jawaban_siswa", upName, upMime, chosenBytes); err != nil {
			return err
		}
		_ = mw.WriteField("email", email)
		_ = mw.WriteField("nama", nama)
	}
	return nil
}

func firstNonEmptyString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			s := strings.TrimSpace(fmt.Sprint(v))
			if s != "" && s != "<nil>" {
				return s
			}
		}
	}
	return ""
}

func formatNumericPercent(m map[string]any, keys ...string) string {
	for _, k := range keys {
		v, ok := m[k]
		if !ok || v == nil {
			continue
		}
		switch num := v.(type) {
		case float64:
			if num >= 0 && num <= 1 && (k == "skor" || k == "score" || k == "probability" || k == "probabilitas" || k == "similarity") {
				return fmt.Sprintf("%.0f%%", num*100)
			}
			return fmt.Sprintf("%.0f%%", num)
		case string:
			if s := strings.TrimSpace(num); s != "" {
				return s
			}
		}
	}
	return ""
}

func extractNoteOrEvidence(m map[string]any, keys ...string) string {
	for _, k := range keys {
		v, ok := m[k]
		if !ok || v == nil {
			continue
		}
		switch val := v.(type) {
		case []any:
			parts := make([]string, 0, len(val))
			for _, item := range val {
				if s := strings.TrimSpace(fmt.Sprint(item)); s != "" && s != "<nil>" {
					parts = append(parts, s)
				}
			}
			if len(parts) > 0 {
				return strings.Join(parts, "; ")
			}
		case string:
			if s := strings.TrimSpace(val); s != "" {
				return s
			}
		default:
			if s := strings.TrimSpace(fmt.Sprint(val)); s != "" && s != "<nil>" {
				return s
			}
		}
	}
	return ""
}

func extractAssessmentSummary(raw json.RawMessage, isSimilarity bool) ([]string, []string) {
	analisa := []string{}
	ringkasan := []string{}
	if len(raw) == 0 {
		return analisa, ringkasan
	}

	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return analisa, ringkasan
	}

	if rawAnalisa, ok := parsed["deskripsi_hasil_analisa"].([]any); ok {
		for _, item := range rawAnalisa {
			if s := strings.TrimSpace(fmt.Sprint(item)); s != "" && s != "<nil>" {
				analisa = append(analisa, s)
			}
		}
	} else if s := firstNonEmptyString(parsed, "ringkasan", "deskripsi_singkat", "kesimpulan"); s != "" {
		analisa = append(analisa, s)
	}

	if isSimilarity {
		// Map per-student similarity keterangan from "siswa" array if available.
		studentNotes := make(map[string]string)
		if siswaList, ok := parsed["siswa"].([]any); ok {
			for _, item := range siswaList {
				if row, ok := item.(map[string]any); ok {
					n := strings.ToLower(firstNonEmptyString(row, "nama", "email"))
					ket := firstNonEmptyString(row, "keterangan", "label", "kategori", "status")
					if n != "" && ket != "" {
						studentNotes[n] = ket
					}
				}
			}
		}

		for _, listKey := range []string{"pasangan", "pairs", "kemiripan", "hasil", "siswa"} {
			list, ok := parsed[listKey].([]any)
			if !ok || len(list) == 0 {
				continue
			}
			for _, item := range list {
				row, ok := item.(map[string]any)
				if !ok {
					continue
				}
				a := firstNonEmptyString(row, "siswa_a", "nama_a", "siswa_1", "nama_1", "nama", "email_a", "email")
				b := firstNonEmptyString(row, "siswa_b", "nama_b", "siswa_2", "nama_2", "paling_mirip_dengan", "email_b")
				pct := formatNumericPercent(row, "persentase", "kemiripan", "persentase_kemiripan", "kemiripan_persen", "skor_kemiripan", "similarity", "skor", "score")
				label := firstNonEmptyString(row, "label", "kategori", "status", "tingkat")
				note := extractNoteOrEvidence(row, "keterangan", "alasan", "catatan", "penjelasan", "saran")
				if note == "" && a != "" {
					note = studentNotes[strings.ToLower(a)]
				}
				parts := []string{}
				if a != "" && b != "" {
					parts = append(parts, fmt.Sprintf("%s ↔ %s", a, b))
				} else if a != "" {
					parts = append(parts, a)
				}
				if pct != "" {
					parts = append(parts, pct)
				}
				if label != "" {
					parts = append(parts, fmt.Sprintf("(%s)", label))
				}
				line := strings.Join(parts, " ")
				if note != "" {
					if line != "" {
						line += " — " + note
					} else {
						line = note
					}
				}
				if line != "" {
					ringkasan = append(ringkasan, line)
				}
			}
			if len(ringkasan) > 0 {
				break
			}
		}
	} else {
		for _, listKey := range []string{"siswa", "hasil", "indikasi", "items"} {
			list, ok := parsed[listKey].([]any)
			if !ok || len(list) == 0 {
				continue
			}
			for _, item := range list {
				row, ok := item.(map[string]any)
				if !ok {
					continue
				}
				nama := firstNonEmptyString(row, "nama", "siswa", "email")
				pct := formatNumericPercent(row, "persentase", "persentase_indikasi_ai", "persentase_ai", "skor_ai", "indikasi_persen", "probability", "probabilitas", "skor", "score")
				label := firstNonEmptyString(row, "label", "band", "indikasi", "kategori", "status")
				note := extractNoteOrEvidence(row, "bukti", "keterangan", "alasan", "catatan", "penjelasan", "saran")
				parts := []string{}
				if nama != "" {
					parts = append(parts, nama+":")
				}
				if pct != "" {
					parts = append(parts, pct)
				}
				if label != "" {
					parts = append(parts, fmt.Sprintf("(%s)", label))
				}
				line := strings.Join(parts, " ")
				if note != "" {
					if line != "" {
						line += " — " + note
					} else {
						line = note
					}
				}
				if line != "" {
					ringkasan = append(ringkasan, line)
				}
			}
			if len(ringkasan) > 0 {
				break
			}
		}
	}

	return analisa, ringkasan
}

func (h *Handler) runAssessmentCheck(
	c *echo.Context,
	upstreamPath string,
	logFitur string,
	defaultFileName string,
	minSubs int,
	minSubsErr string,
	isSimilarity bool,
) error {
	claims, err := utils.GetClaims(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "Tidak dapat mengakses data klaim")
	}

	apiKey := h.effectiveAPIKey()
	if apiKey == "" {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "Layanan AI belum dikonfigurasi di server")
	}

	req := new(AssessmentCheckRequest)
	if err := c.Bind(req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if err := c.Validate(req); err != nil {
		return utils.ValidationErrorHandler(c, err)
	}

	selectedIDs := make(map[int32]bool, len(req.PenggunaIDs))
	for _, id := range req.PenggunaIDs {
		if id > 0 {
			selectedIDs[id] = true
		}
	}

	ctx := c.Request().Context()
	ac, err := h.loadAssessmentContext(ctx, claims, strings.TrimSpace(req.KodeKelas), req.PostinganID, selectedIDs, minSubs, minSubsErr)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	mapel := strings.TrimSpace(ac.kelasRow.Nama)
	_ = mw.WriteField("pengajar", ac.pengajarNama)
	_ = mw.WriteField("mata_pelajaran", mapel)
	_ = mw.WriteField("kelas", strings.TrimSpace(ac.kelasRow.Nama))
	_ = mw.WriteField("judul_tugas", strings.TrimSpace(ac.postRow.Judul))
	if !isSimilarity && strings.TrimSpace(req.Jenjang) != "" {
		_ = mw.WriteField("jenjang", strings.TrimSpace(req.Jenjang))
	}
	_ = mw.WriteField("gunakan_model_gratis", strconv.FormatBool(req.GunakanModelGratis))

	if err := appendSubmissionsMultipart(mw, ac.targetSubs); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal menyiapkan berkas jawaban siswa")
	}
	if err := mw.Close(); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal menyiapkan permintaan analisis AI")
	}

	actorID := actorIDForClaims(claims)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, h.effectiveBaseURL()+upstreamPath, &buf)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal membuat permintaan analisis AI")
	}
	httpReq.Header.Set("Content-Type", mw.FormDataContentType())
	httpReq.Header.Set("X-API-Key", apiKey)
	httpReq.Header.Set("X-Actor-Id", actorID)

	resp, err := h.httpClient.Do(httpReq)
	if err != nil {
		log.Println("ai assessment check error:", err)
		return echo.NewHTTPError(http.StatusBadGateway, "Tidak dapat terhubung ke server AI")
	}
	body, readErr := io.ReadAll(resp.Body)
	resp.Body.Close()
	if readErr != nil {
		return echo.NewHTTPError(http.StatusBadGateway, "Gagal membaca respons server AI")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return echo.NewHTTPError(http.StatusBadGateway, extractRemoteAIError(body, resp.StatusCode))
	}

	var created remoteJobEnvelope
	if err := json.Unmarshal(body, &created); err != nil || strings.TrimSpace(created.JobID) == "" {
		return echo.NewHTTPError(http.StatusBadGateway, "Job ID analisis AI tidak valid")
	}

	finalJob, err := h.pollJob(ctx, created.JobID, actorID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadGateway, err.Error())
	}

	analisa, ringkasan := extractAssessmentSummary(finalJob.Result, isSimilarity)

	var rekapFile *files.UploadedPostinganFile
	if finalJob.Artifact != nil && strings.TrimSpace(finalJob.Artifact.Availability) != "" {
		if artifactBytes, err := h.downloadArtifact(ctx, finalJob.Artifact.Availability, actorID); err == nil {
			fileName := strings.TrimSpace(finalJob.Artifact.Name)
			if fileName == "" {
				fileName = defaultFileName
			}
			if saved, err := files.SaveUploadedBytes(fileName, path.Ext(fileName), artifactBytes); err == nil {
				rekapFile = &saved
			}
		}
	}

	h.logUsage(ctx, claims.UserID, logFitur)
	return c.JSON(http.StatusOK, AssessmentCheckResponse{
		RekapFile: rekapFile,
		Analisa:   analisa,
		Ringkasan: ringkasan,
	})
}

// CekKemiripan godoc
//
//	@Summary		Periksa Kemiripan Jawaban Tugas Siswa (Guru)
//	@Description	Memeriksa indikasi kemiripan/plagiasi antarjawaban siswa pada suatu tugas melalui ai.pathclass.app /v1/assessments/similarity
//	@Tags			AI
//	@Accept			json
//	@Produce		json
//	@Param			request						body		AssessmentCheckRequest	true	"Permintaan Cek Kemiripan AI"
//	@Success		200							{object}	AssessmentCheckResponse
//	@Failure		400,401,403,404,500,502,503	{object}	utils.HTTPResponse
//	@Router			/ai/similarity [post]
func (h *Handler) CekKemiripan(c *echo.Context) error {
	return h.runAssessmentCheck(
		c,
		"/v1/assessments/similarity",
		"similarity_ai",
		"rekap-kemiripan-ai.xlsx",
		2,
		"Minimal 2 siswa yang mengumpulkan tugas diperlukan untuk cek kemiripan",
		true,
	)
}

// CekIndikasiAI godoc
//
//	@Summary		Periksa Indikasi Penggunaan AI pada Jawaban Siswa (Guru)
//	@Description	Memeriksa indikasi penggunaan AI pada jawaban tugas siswa melalui ai.pathclass.app /v1/assessments/ai-usage
//	@Tags			AI
//	@Accept			json
//	@Produce		json
//	@Param			request						body		AssessmentCheckRequest	true	"Permintaan Cek Indikasi AI"
//	@Success		200							{object}	AssessmentCheckResponse
//	@Failure		400,401,403,404,500,502,503	{object}	utils.HTTPResponse
//	@Router			/ai/indikasi [post]
func (h *Handler) CekIndikasiAI(c *echo.Context) error {
	return h.runAssessmentCheck(
		c,
		"/v1/assessments/ai-usage",
		"indikasi_ai",
		"laporan-indikasi-ai.pdf",
		1,
		"Belum ada kiriman siswa untuk diperiksa indikasi AI",
		false,
	)
}

// RingkasDokumen godoc
//
//	@Summary		Ringkas Dokumen Materi Panjang dengan AI
//	@Description	Meringkas dokumen materi (PDF, DOCX, TXT, CSV) yang terlampir pada materi kelas melalui ai.pathclass.app /v1/documents/summarize
//	@Tags			AI
//	@Accept			json
//	@Produce		json
//	@Param			request				body		RingkasDokumenRequest	true	"Permintaan Ringkas Dokumen AI"
//	@Success		200					{object}	ExplainMateriResponse
//	@Failure		400,401,500,502,503	{object}	utils.HTTPResponse
//	@Router			/ai/dokumen/ringkas [post]
func (h *Handler) RingkasDokumen(c *echo.Context) error {
	claims, err := utils.GetClaims(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "Tidak dapat mengakses data klaim")
	}

	apiKey := h.effectiveAPIKey()
	if apiKey == "" {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "Layanan AI belum dikonfigurasi di server")
	}

	req := new(RingkasDokumenRequest)
	if err := c.Bind(req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if err := c.Validate(req); err != nil {
		return utils.ValidationErrorHandler(c, err)
	}

	var docName string
	var docBytes []byte
	for _, rawPath := range req.LampiranPaths {
		if name, data, ok := readSafeUploadFile(rawPath); ok && isPreferredAnswerDoc(name) {
			docName = name
			docBytes = data
			break
		}
	}
	if len(docBytes) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Tidak ada dokumen (PDF, DOCX, atau TXT) yang dapat diringkas")
	}

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	upName, upMime := mimeForFilename(docName)
	if err := writeMultipartFile(mw, "berkas_dokumen", upName, upMime, docBytes); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal menyiapkan berkas dokumen")
	}
	_ = mw.WriteField("judul", strings.TrimSpace(req.Judul))
	_ = mw.WriteField("fokus", strings.TrimSpace(req.Fokus))
	_ = mw.WriteField("jenjang", strings.TrimSpace(req.Jenjang))
	_ = mw.WriteField("diagram", strconv.FormatBool(req.Diagram))
	_ = mw.WriteField("gunakan_model_gratis", strconv.FormatBool(req.GunakanModelGratis))
	if err := mw.Close(); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal menyiapkan permintaan ringkasan AI")
	}

	// ai.pathclass.app restricts /v1/documents/summarize to guru: actors;
	// use guru:<userID> upstream while logging under claims.UserID locally.
	actorID := fmt.Sprintf("guru:%d", claims.UserID)
	ctx := c.Request().Context()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, h.effectiveBaseURL()+"/v1/documents/summarize", &buf)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal membuat permintaan ringkasan AI")
	}
	httpReq.Header.Set("Content-Type", mw.FormDataContentType())
	httpReq.Header.Set("X-API-Key", apiKey)
	httpReq.Header.Set("X-Actor-Id", actorID)

	resp, err := h.httpClient.Do(httpReq)
	if err != nil {
		log.Println("ai summarize document error:", err)
		return echo.NewHTTPError(http.StatusBadGateway, "Tidak dapat terhubung ke server AI")
	}
	body, readErr := io.ReadAll(resp.Body)
	resp.Body.Close()
	if readErr != nil {
		return echo.NewHTTPError(http.StatusBadGateway, "Gagal membaca respons server AI")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return echo.NewHTTPError(http.StatusBadGateway, extractRemoteAIError(body, resp.StatusCode))
	}

	var created remoteJobEnvelope
	if err := json.Unmarshal(body, &created); err != nil || strings.TrimSpace(created.JobID) == "" {
		return echo.NewHTTPError(http.StatusBadGateway, "Job ID ringkasan AI tidak valid")
	}

	finalJob, err := h.pollJob(ctx, created.JobID, actorID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadGateway, err.Error())
	}

	var sumResult struct {
		Judul              string   `json:"judul"`
		DeskripsiSingkat   string   `json:"deskripsi_singkat"`
		Ringkasan          string   `json:"ringkasan"`
		RingkasanEksekutif string   `json:"ringkasan_eksekutif"`
		TemuanUtama        []string `json:"temuan_utama"`
	}
	_ = json.Unmarshal(finalJob.Result, &sumResult)

	judul := strings.TrimSpace(sumResult.Judul)
	if judul == "" {
		judul = strings.TrimSpace(req.Judul)
	}
	jawaban := strings.TrimSpace(sumResult.Ringkasan)
	if jawaban == "" {
		jawaban = strings.TrimSpace(sumResult.RingkasanEksekutif)
	}
	if jawaban == "" {
		jawaban = strings.TrimSpace(sumResult.DeskripsiSingkat)
	}

	// If job.result only contains document metadata, fetch the JSON summary artifact (telaah-dokumen.json) if available.
	if jawaban == "" && finalJob.Artifact != nil && finalJob.Artifact.JSON != nil && strings.TrimSpace(finalJob.Artifact.JSON.Availability) != "" {
		if jsonBytes, err := h.downloadArtifact(ctx, finalJob.Artifact.JSON.Availability, actorID); err == nil {
			var detail struct {
				Judul              string   `json:"judul"`
				RingkasanEksekutif string   `json:"ringkasan_eksekutif"`
				TemuanUtama        []string `json:"temuan_utama"`
			}
			if json.Unmarshal(jsonBytes, &detail) == nil {
				if judul == "" && strings.TrimSpace(detail.Judul) != "" {
					judul = strings.TrimSpace(detail.Judul)
				}
				parts := make([]string, 0, 2)
				if re := strings.TrimSpace(detail.RingkasanEksekutif); re != "" {
					parts = append(parts, re)
				}
				if len(detail.TemuanUtama) > 0 {
					bullets := make([]string, 0, len(detail.TemuanUtama))
					for _, t := range detail.TemuanUtama {
						if trimmed := strings.TrimSpace(t); trimmed != "" {
							bullets = append(bullets, "- "+trimmed)
						}
					}
					if len(bullets) > 0 {
						parts = append(parts, "**Temuan Utama:**\n"+strings.Join(bullets, "\n"))
					}
				}
				jawaban = strings.TrimSpace(strings.Join(parts, "\n\n"))
			}
		}
	}

	if jawaban == "" {
		jawaban = "Ringkasan dokumen AI telah selesai dibuat. Buka dokumen PDF di bawah ini untuk melihat ringkasan lengkapnya."
	}

	var savedFile *files.UploadedPostinganFile
	if finalJob.Artifact != nil && strings.TrimSpace(finalJob.Artifact.Availability) != "" {
		artifactBytes, err := h.downloadArtifact(ctx, finalJob.Artifact.Availability, actorID)
		if err != nil {
			log.Println("ai download summarize artifact error:", err)
			return echo.NewHTTPError(http.StatusBadGateway, "Gagal mengunduh file ringkasan AI")
		}
		fileName := strings.TrimSpace(finalJob.Artifact.Name)
		if fileName == "" {
			fileName = "ringkasan-dokumen-ai.pdf"
		}
		saved, err := files.SaveUploadedBytes(fileName, path.Ext(fileName), artifactBytes)
		if err != nil {
			log.Println("ai save summarize artifact error:", err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Gagal menyimpan file ringkasan AI")
		}
		savedFile = &saved
	}

	h.logUsage(ctx, claims.UserID, "bantuan_ai")
	return c.JSON(http.StatusOK, ExplainMateriResponse{
		Judul:   judul,
		Jawaban: jawaban,
		File:    savedFile,
	})
}

// OpsUsage godoc
//
//	@Summary		Status Pemakaian Token & Kuota AI (Admin)
//	@Description	Mengambil informasi sisa anggaran token berbayar dan kuota model gratis dari ai.pathclass.app /v1/ops/usage (khusus admin)
//	@Tags			AI
//	@Produce		json
//	@Success		200					{object}	map[string]any
//	@Failure		401,403,502,503		{object}	utils.HTTPResponse
//	@Router			/ai/ops/usage [get]
func (h *Handler) OpsUsage(c *echo.Context) error {
	claims, err := utils.GetClaims(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "Tidak dapat mengakses data klaim")
	}
	if !claims.IsAdmin() {
		return echo.NewHTTPError(http.StatusForbidden, "Hanya admin yang dapat melihat pemakaian token AI")
	}

	apiKey := h.effectiveAPIKey()
	if apiKey == "" {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "Layanan AI belum dikonfigurasi di server")
	}

	httpReq, err := http.NewRequestWithContext(
		c.Request().Context(),
		http.MethodGet,
		h.effectiveBaseURL()+"/v1/ops/usage",
		nil,
	)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal membuat permintaan status AI")
	}
	httpReq.Header.Set("X-API-Key", apiKey)
	httpReq.Header.Set("X-Actor-Id", actorIDForClaims(claims))

	resp, err := h.httpClient.Do(httpReq)
	if err != nil {
		log.Println("ai ops usage error:", err)
		return echo.NewHTTPError(http.StatusBadGateway, "Tidak dapat terhubung ke server AI")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadGateway, "Gagal membaca respons server AI")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return echo.NewHTTPError(http.StatusBadGateway, extractRemoteAIError(body, resp.StatusCode))
	}

	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err != nil {
		return echo.NewHTTPError(http.StatusBadGateway, "Respons pemakaian token AI tidak valid")
	}
	return c.JSON(http.StatusOK, parsed)
}
