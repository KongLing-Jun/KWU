import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  AlertCircle,
  CheckCircle2,
  ImagePlus,
  LoaderCircle,
  LogIn,
  ShieldCheck,
  Trash2,
} from "lucide-react";

import { CopyButton } from "../../../components/CopyButton";
import { app } from "../../../lib/bridge";
import { useT } from "../../../lib/i18n";
import type {
  FeedbackImageView,
  FeedbackOperationErrorView,
  FeedbackSubmitView,
} from "../../../lib/types";

const FEEDBACK_DESCRIPTION_MAX_RUNES = 300;
const FEEDBACK_DESCRIPTION_MAX_BYTES = 10_000;
const FEEDBACK_IMAGE_LIMIT = 4;
const FEEDBACK_IMAGE_MAX_BYTES = 5 * 1024 * 1024;
const FEEDBACK_THUMBNAIL_MAX_DATA_URL_BYTES = 256 * 1024;
const FEEDBACK_TOKEN_PATTERN = /^feedback:[a-f0-9]{48}$/;
const SAFE_IDENTIFIER_PATTERN = /^[A-Za-z0-9_.:-]+$/;
const CHINESE_MESSAGE_PATTERN = /[\u3400-\u9fff]/;

export type FeedbackAccountState = "loading" | "ready" | "anonymous" | "expired" | "unavailable";

export function resolveFeedbackAccountState({
  initialized,
  available,
  status,
  hasRecoverableIdentity,
}: {
  initialized: boolean;
  available: boolean;
  status?: string;
  hasRecoverableIdentity: boolean;
}): FeedbackAccountState {
  if (!initialized) return "loading";
  if (!available) return "unavailable";
  if (status === "expired") return "expired";
  if (hasRecoverableIdentity && (status === "authenticated" || status === "offline")) return "ready";
  return "anonymous";
}

type FeedbackSuccess = {
  reportID: string;
  replayed: boolean;
};

export type FeedbackDraftController = {
  accountState: FeedbackAccountState;
  description: string;
  images: FeedbackImageView[];
  operationError: FeedbackOperationErrorView | null;
  success: FeedbackSuccess | null;
  notice: string | null;
  submitting: boolean;
  pickingImages: boolean;
  removingToken: string | null;
  descriptionRunes: number;
  descriptionError: string | null;
  dirty: boolean;
  canSubmit: boolean;
  setDescription: (value: string) => void;
  pickImages: () => Promise<void>;
  removeImage: (token: string) => Promise<void>;
  submit: () => Promise<void>;
  requestClose: (close: () => void) => Promise<void>;
  requestAccount: () => Promise<void>;
};

export function useFeedbackDraft({
  accountState,
  accountScopeKey,
  onRequireAccount,
}: {
  accountState: FeedbackAccountState;
  accountScopeKey: string;
  onRequireAccount: () => void;
}): FeedbackDraftController {
  const t = useT();
  const [description, setDescriptionState] = useState("");
  const [images, setImages] = useState<FeedbackImageView[]>([]);
  const [operationError, setOperationError] = useState<FeedbackOperationErrorView | null>(null);
  const [success, setSuccess] = useState<FeedbackSuccess | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [pickingImages, setPickingImages] = useState(false);
  const [removingToken, setRemovingToken] = useState<string | null>(null);
  const imagesRef = useRef<FeedbackImageView[]>([]);
  const descriptionRef = useRef("");
  const attemptIDRef = useRef<string | null>(null);
  const lifecycleRef = useRef(0);
  const identityKey = accountState === "ready" ? accountScopeKey : "";
  const identityKeyRef = useRef(identityKey);

  imagesRef.current = images;
  descriptionRef.current = description;

  const clearResultForEdit = useCallback(() => {
    attemptIDRef.current = null;
    setOperationError(null);
    setSuccess(null);
    setNotice(null);
  }, []);

  const resetDraftState = useCallback(() => {
    descriptionRef.current = "";
    imagesRef.current = [];
    attemptIDRef.current = null;
    setDescriptionState("");
    setImages([]);
    setOperationError(null);
    setSuccess(null);
    setRemovingToken(null);
  }, []);

  const discardTokens = useCallback(async (tokens: string[]) => {
    if (tokens.length === 0) return;
    try {
      await app.DiscardFeedbackDraft(tokens);
    } catch {
      // Logout, account switch, or shutdown may already have released them.
    }
  }, []);

  useEffect(() => {
    if (identityKeyRef.current === identityKey) return;
    identityKeyRef.current = identityKey;
    lifecycleRef.current += 1;
    const staleTokens = imagesRef.current.map((image) => image.token);
    const hadDraft = descriptionRef.current.length > 0 || staleTokens.length > 0;
    void discardTokens(staleTokens);
    resetDraftState();
    setSubmitting(false);
    setPickingImages(false);
    if (hadDraft) setNotice(t("feedback.scopeChanged"));
  }, [discardTokens, identityKey, resetDraftState, t]);

  useEffect(() => {
    return () => {
      lifecycleRef.current += 1;
      void discardTokens(imagesRef.current.map((image) => image.token));
    };
  }, [discardTokens]);

  const setDescription = useCallback((value: string) => {
    descriptionRef.current = value;
    setDescriptionState(value);
    clearResultForEdit();
  }, [clearResultForEdit]);

  const descriptionRunes = Array.from(description).length;
  const descriptionBytes = useMemo(() => new TextEncoder().encode(description.trim()).length, [description]);
  const descriptionError = useMemo(() => {
    if (descriptionRunes > FEEDBACK_DESCRIPTION_MAX_RUNES) return t("feedback.descriptionTooLong");
    if (descriptionBytes > FEEDBACK_DESCRIPTION_MAX_BYTES) return t("feedback.descriptionBytesTooLong");
    return null;
  }, [descriptionBytes, descriptionRunes, t]);
  const dirty = description.length > 0 || images.length > 0;

  const pickImages = useCallback(async () => {
    if (accountState !== "ready" || submitting || pickingImages || imagesRef.current.length >= FEEDBACK_IMAGE_LIMIT) return;
    setPickingImages(true);
    setOperationError(null);
    setNotice(null);
    const existingTokens = imagesRef.current.map((image) => image.token);
    const lifecycle = lifecycleRef.current;
    let selectedTokens: string[] = [];
    try {
      const selection = await app.PickFeedbackImages(existingTokens);
      selectedTokens = feedbackTokensFromSelection(selection?.images, new Set(existingTokens));
      if (lifecycleRef.current !== lifecycle) {
        await discardTokens(selectedTokens);
        return;
      }
      const nextImages = normalizeFeedbackSelection(selection?.images, new Set(existingTokens));
      if (imagesRef.current.length + nextImages.length > FEEDBACK_IMAGE_LIMIT) {
        throw new Error("feedback image limit exceeded");
      }
      if (nextImages.length > 0) {
        setImages((current) => {
          const next = [...current, ...nextImages];
          imagesRef.current = next;
          return next;
        });
        clearResultForEdit();
      }
    } catch {
      await discardTokens(selectedTokens);
      if (lifecycleRef.current === lifecycle) {
        setOperationError(localFeedbackError("FEEDBACK_IMAGE_PICK_FAILED", t("feedback.imagePickFailed")));
      }
    } finally {
      if (lifecycleRef.current === lifecycle) setPickingImages(false);
    }
  }, [accountState, clearResultForEdit, discardTokens, pickingImages, submitting, t]);

  const removeImage = useCallback(async (token: string) => {
    if (submitting || removingToken !== null || !FEEDBACK_TOKEN_PATTERN.test(token)) return;
    const lifecycle = lifecycleRef.current;
    setRemovingToken(token);
    setOperationError(null);
    try {
      await app.RemoveFeedbackImage(token);
      if (lifecycleRef.current !== lifecycle) return;
      setImages((current) => {
        const next = current.filter((image) => image.token !== token);
        imagesRef.current = next;
        return next;
      });
      clearResultForEdit();
    } catch {
      if (lifecycleRef.current === lifecycle) {
        setOperationError(localFeedbackError("FEEDBACK_IMAGE_REMOVE_FAILED", t("feedback.imageRemoveFailed")));
      }
    } finally {
      if (lifecycleRef.current === lifecycle) setRemovingToken(null);
    }
  }, [clearResultForEdit, removingToken, submitting, t]);

  const submit = useCallback(async () => {
    const trimmed = descriptionRef.current.trim();
    const runes = Array.from(trimmed).length;
    const bytes = new TextEncoder().encode(trimmed).length;
    if (
      accountState !== "ready"
      || submitting
      || trimmed.length === 0
      || runes > FEEDBACK_DESCRIPTION_MAX_RUNES
      || bytes > FEEDBACK_DESCRIPTION_MAX_BYTES
      || imagesRef.current.length > FEEDBACK_IMAGE_LIMIT
    ) {
      if (trimmed.length === 0) {
        setOperationError(localFeedbackError("FEEDBACK_DESCRIPTION_REQUIRED", t("feedback.descriptionRequired")));
      }
      return;
    }
    const attemptID = attemptIDRef.current ?? newFeedbackAttemptID();
    const lifecycle = lifecycleRef.current;
    attemptIDRef.current = attemptID;
    setSubmitting(true);
    setOperationError(null);
    setSuccess(null);
    setNotice(null);
    try {
      const result = await app.SubmitFeedback({
        description: trimmed,
        attempt_id: attemptID,
        image_tokens: imagesRef.current.map((image) => image.token),
      });
      if (lifecycleRef.current !== lifecycle) return;
      const normalized = normalizeFeedbackSubmission(result, t("feedback.submitFailed"));
      if (normalized.operationError) {
        setOperationError(normalized.operationError);
        return;
      }
      if (!normalized.success) {
        setOperationError(localFeedbackError("FEEDBACK_ERROR", t("feedback.submitFailed"), true));
        return;
      }
      resetDraftState();
      setNotice(null);
      setSuccess(normalized.success);
    } catch {
      if (lifecycleRef.current === lifecycle) {
        setOperationError(localFeedbackError("FEEDBACK_ERROR", t("feedback.submitFailed"), true));
      }
    } finally {
      if (lifecycleRef.current === lifecycle) setSubmitting(false);
    }
  }, [accountState, resetDraftState, submitting, t]);

  const confirmDiscard = useCallback(async (): Promise<boolean> => {
    if (!dirty) return true;
    try {
      return await app.ConfirmAction({
        title: t("feedback.discardTitle"),
        message: t("feedback.discardMessage"),
        detail: t("feedback.discardDetail"),
        confirmLabel: t("feedback.discardConfirm"),
        cancelLabel: t("common.cancel"),
        destructive: true,
      });
    } catch {
      return false;
    }
  }, [dirty, t]);

  const discardCurrentDraft = useCallback(async () => {
    const tokens = imagesRef.current.map((image) => image.token);
    await discardTokens(tokens);
    resetDraftState();
  }, [discardTokens, resetDraftState]);

  const requestClose = useCallback(async (close: () => void) => {
    if (submitting || !(await confirmDiscard())) return;
    await discardCurrentDraft();
    close();
  }, [confirmDiscard, discardCurrentDraft, submitting]);

  const requestAccount = useCallback(async () => {
    if (submitting || !(await confirmDiscard())) return;
    await discardCurrentDraft();
    onRequireAccount();
  }, [confirmDiscard, discardCurrentDraft, onRequireAccount, submitting]);

  return {
    accountState,
    description,
    images,
    operationError,
    success,
    notice,
    submitting,
    pickingImages,
    removingToken,
    descriptionRunes,
    descriptionError,
    dirty,
    canSubmit: accountState === "ready"
      && description.trim().length > 0
      && descriptionError === null
      && !submitting
      && !pickingImages
      && removingToken === null,
    setDescription,
    pickImages,
    removeImage,
    submit,
    requestClose,
    requestAccount,
  };
}

export function FeedbackSettingsSection({ controller }: { controller: FeedbackDraftController }) {
  const t = useT();
  const {
    accountState,
    description,
    images,
    operationError,
    success,
    notice,
    submitting,
    pickingImages,
    removingToken,
    descriptionRunes,
    descriptionError,
    canSubmit,
  } = controller;
  const formDisabled = accountState !== "ready" || submitting;
  const accountMessage = feedbackAccountMessage(accountState, t);
  const showAccountAction = accountState === "anonymous" || accountState === "expired";

  return (
    <section className="feedback-settings" aria-busy={submitting}>
      {accountState !== "ready" && (
        <div className="feedback-settings__account" role="status">
          <div>
            <strong>{accountMessage.title}</strong>
            <p>{accountMessage.detail}</p>
          </div>
          {showAccountAction && (
            <button className="btn btn--small" type="button" onClick={() => void controller.requestAccount()} disabled={submitting}>
              <LogIn size={14} aria-hidden="true" />
              {t("feedback.goLogin")}
            </button>
          )}
        </div>
      )}

      {notice && <div className="banner banner--warning feedback-settings__banner" role="status">{notice}</div>}
      {operationError && <FeedbackError error={operationError} />}
      {success && <FeedbackSuccessView success={success} />}

      <div className="feedback-settings__composer">
        <div className="feedback-settings__label-row">
          <label htmlFor="feedback-description">{t("feedback.descriptionLabel")}</label>
        </div>
        <div className="feedback-settings__textarea-wrap">
          <textarea
            id="feedback-description"
            className="feedback-settings__textarea"
            value={description}
            rows={7}
            disabled={formDisabled}
            aria-invalid={descriptionError ? "true" : undefined}
            aria-describedby="feedback-description-hint"
            placeholder={t("feedback.descriptionPlaceholder")}
            onChange={(event) => controller.setDescription(event.currentTarget.value)}
          />

          {images.length > 0 && (
            <ul className="feedback-settings__image-list" aria-label={t("feedback.imagesLabel")}>
              {images.map((image) => (
                <li className="feedback-settings__image" key={image.token}>
                  <img src={image.thumbnail_data_url} alt="" />
                  <div className="feedback-settings__image-copy">
                    <strong title={image.filename}>{image.filename}</strong>
                    <span>{formatFeedbackBytes(image.size_bytes)}</span>
                  </div>
                  <button
                    className="feedback-settings__image-remove"
                    type="button"
                    aria-label={t("feedback.imageRemove", { name: image.filename })}
                    disabled={submitting || removingToken !== null}
                    onClick={() => void controller.removeImage(image.token)}
                  >
                    {removingToken === image.token
                      ? <LoaderCircle className="feedback-settings__spinner" size={14} aria-hidden="true" />
                      : <Trash2 size={14} aria-hidden="true" />}
                  </button>
                </li>
              ))}
            </ul>
          )}

          <div className="feedback-settings__textarea-bottom">
            <button
              className="feedback-settings__image-button"
              type="button"
              disabled={formDisabled || pickingImages || images.length >= FEEDBACK_IMAGE_LIMIT}
              onClick={() => void controller.pickImages()}
            >
              {pickingImages ? <LoaderCircle className="feedback-settings__spinner" size={15} aria-hidden="true" /> : <ImagePlus size={15} aria-hidden="true" />}
              {pickingImages ? t("feedback.imagePicking") : t("feedback.imageAdd")}
            </button>
            <span className={descriptionError ? "feedback-settings__count feedback-settings__count--error" : "feedback-settings__count"}>
              {descriptionRunes}/{FEEDBACK_DESCRIPTION_MAX_RUNES}
            </span>
          </div>
        </div>
        <div id="feedback-description-hint" className={descriptionError ? "feedback-settings__hint feedback-settings__hint--error" : "feedback-settings__hint"}>
          {descriptionError ?? t("feedback.descriptionHint")}
        </div>

        <p className="feedback-settings__images-hint">{t("feedback.imagesHint")}</p>

        <div className="feedback-settings__footer">
          <div className="feedback-settings__privacy">
            <ShieldCheck size={16} aria-hidden="true" />
            <span>{t("feedback.privacyHint")}</span>
          </div>
          <button
            className="feedback-settings__submit"
            type="button"
            disabled={!canSubmit}
            onClick={() => void controller.submit()}
          >
            {submitting && <LoaderCircle className="feedback-settings__spinner" size={15} aria-hidden="true" />}
            {submitting ? t("feedback.submitting") : t("feedback.submit")}
          </button>
        </div>

        <p className="feedback-settings__submit-hint">{t("feedback.submitHint")}</p>
      </div>
    </section>
  );
}

function FeedbackError({ error }: { error: FeedbackOperationErrorView }) {
  const t = useT();
  return (
    <div className="feedback-settings__result feedback-settings__result--error" role="alert">
      <AlertCircle size={18} aria-hidden="true" />
      <div>
        <strong>{t("feedback.errorTitle")}</strong>
        <p>{error.message}</p>
        <div className="feedback-settings__result-meta">
          <code>{error.code}</code>
          {error.request_id && (
            <span>
              {t("feedback.requestID")}: <code>{error.request_id}</code>
              <CopyButton text={error.request_id} showInlineLabel={false} label={t("common.copy")} />
            </span>
          )}
          {error.diagnostic_id && (
            <span data-feedback-error-log-id translate="no">
              {t("feedback.logID")}: <code>{error.diagnostic_id}</code>
              <CopyButton text={error.diagnostic_id} showInlineLabel={false} label={t("common.copy")} />
            </span>
          )}
        </div>
      </div>
    </div>
  );
}

function FeedbackSuccessView({ success }: { success: FeedbackSuccess }) {
  const t = useT();
  return (
    <div className="feedback-settings__result feedback-settings__result--success" role="status">
      <CheckCircle2 size={18} aria-hidden="true" />
      <div>
        <strong>{t("feedback.successTitle")}</strong>
        <p>{success.replayed ? t("feedback.successReplayed") : t("feedback.successDetail")}</p>
        <div className="feedback-settings__report">
          <span>{t("feedback.reportID")}</span>
          <code>{success.reportID}</code>
          <CopyButton text={success.reportID} label={t("common.copy")} />
        </div>
      </div>
    </div>
  );
}

function feedbackAccountMessage(accountState: FeedbackAccountState, t: ReturnType<typeof useT>) {
  switch (accountState) {
    case "loading":
      return { title: t("feedback.accountLoading"), detail: t("feedback.accountLoadingDetail") };
    case "expired":
      return { title: t("feedback.accountExpired"), detail: t("feedback.accountExpiredDetail") };
    case "anonymous":
      return { title: t("feedback.accountRequired"), detail: t("feedback.accountRequiredDetail") };
    case "unavailable":
      return { title: t("feedback.unavailable"), detail: t("feedback.unavailableDetail") };
    case "ready":
      return { title: "", detail: "" };
  }
}

function normalizeFeedbackSelection(raw: unknown, existingTokens: Set<string>): FeedbackImageView[] {
  if (!Array.isArray(raw)) throw new Error("invalid feedback image selection");
  const seen = new Set(existingTokens);
  return raw.map((value) => {
    if (value === null || typeof value !== "object") throw new Error("invalid feedback image");
    const image = value as Partial<FeedbackImageView>;
    const token = typeof image.token === "string" ? image.token : "";
    const filename = typeof image.filename === "string" ? image.filename : "";
    const contentType = image.content_type;
    const thumbnail = typeof image.thumbnail_data_url === "string" ? image.thumbnail_data_url : "";
    const size = typeof image.size_bytes === "number" ? image.size_bytes : 0;
    if (
      !FEEDBACK_TOKEN_PATTERN.test(token)
      || seen.has(token)
      || filename.length === 0
      || filename.length > 160
      || /[\\/\r\n\0]/.test(filename)
      || (contentType !== "image/png" && contentType !== "image/jpeg")
      || !thumbnail.startsWith("data:image/jpeg;base64,")
      || thumbnail.length > FEEDBACK_THUMBNAIL_MAX_DATA_URL_BYTES
      || !Number.isSafeInteger(size)
      || size <= 0
      || size > FEEDBACK_IMAGE_MAX_BYTES
    ) {
      throw new Error("invalid feedback image");
    }
    seen.add(token);
    return {
      token,
      filename,
      content_type: contentType,
      thumbnail_data_url: thumbnail,
      size_bytes: size,
    };
  });
}

function feedbackTokensFromSelection(raw: unknown, existingTokens: Set<string>): string[] {
  if (!Array.isArray(raw)) return [];
  const seen = new Set(existingTokens);
  const tokens: string[] = [];
  for (const value of raw) {
    if (value === null || typeof value !== "object") continue;
    const token = typeof (value as Partial<FeedbackImageView>).token === "string"
      ? (value as Partial<FeedbackImageView>).token!
      : "";
    if (!FEEDBACK_TOKEN_PATTERN.test(token) || seen.has(token)) continue;
    seen.add(token);
    tokens.push(token);
  }
  return tokens;
}

function normalizeFeedbackSubmission(result: FeedbackSubmitView | null | undefined, fallbackMessage: string): {
  success?: FeedbackSuccess;
  operationError?: FeedbackOperationErrorView;
} {
  if (result?.operation_error) {
    return { operationError: normalizeFeedbackOperationError(result.operation_error, fallbackMessage) };
  }
  const reportID = typeof result?.report_id === "string" ? result.report_id.trim() : "";
  if (!reportID || reportID.length > 128 || !SAFE_IDENTIFIER_PATTERN.test(reportID)) return {};
  return { success: { reportID, replayed: result?.replayed === true } };
}

function normalizeFeedbackOperationError(error: FeedbackOperationErrorView, fallbackMessage: string): FeedbackOperationErrorView {
  const code = typeof error.code === "string" && error.code.length <= 64 && /^[A-Z0-9_-]+$/.test(error.code)
    ? error.code
    : "FEEDBACK_ERROR";
  const message = typeof error.message === "string" && error.message.length <= 300 && CHINESE_MESSAGE_PATTERN.test(error.message)
    ? error.message
    : fallbackMessage;
  const requestID = typeof error.request_id === "string"
    && error.request_id.length <= 128
    && SAFE_IDENTIFIER_PATTERN.test(error.request_id)
    ? error.request_id
    : undefined;
  const diagnosticID = typeof error.diagnostic_id === "string"
    && error.diagnostic_id.length <= 64
    && /^[A-Za-z0-9_-]+$/.test(error.diagnostic_id)
    ? error.diagnostic_id
    : undefined;
  return { code, message, retryable: error.retryable === true, request_id: requestID, diagnostic_id: diagnosticID };
}

function localFeedbackError(code: string, message: string, retryable = false): FeedbackOperationErrorView {
  return { code, message, retryable };
}

function newFeedbackAttemptID(): string {
  if (typeof crypto.randomUUID === "function") return crypto.randomUUID();
  const bytes = new Uint8Array(16);
  crypto.getRandomValues(bytes);
  return `feedback-${Array.from(bytes, (value) => value.toString(16).padStart(2, "0")).join("")}`;
}

function formatFeedbackBytes(value: number): string {
  if (value >= 1024 * 1024) return `${(value / (1024 * 1024)).toFixed(1)} MiB`;
  return `${Math.max(1, Math.round(value / 1024))} KiB`;
}
