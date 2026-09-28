import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState, type Ref } from "react";
import {
  ArrowLeft,
  ArrowRightLeft,
  ChevronDown,
  CircleAlert,
  Coins,
  Eye,
  EyeOff,
  LogOut,
  RefreshCw,
  ShieldCheck,
  UserRound,
} from "lucide-react";

import {
  type CloudLoginRequest,
  type CloudRegisterRequest,
  type CloudResetPasswordRequest,
} from "../../../lib/bridge";
import {
  AliyunCaptchaAdapter,
  validateAliyunCaptchaConfig,
  type AliyunCaptchaProof,
} from "../../../lib/aliyunCaptcha";
import { BRAND_DISPLAY_NAME } from "../../../lib/brand";
import { clientErrorPresentation, reportClientError } from "../../../lib/clientError";
import { useI18n, useT } from "../../../lib/i18n";
import { CopyButton } from "../../../components/CopyButton";
import {
  cloudScopeKey,
  hasRecoverableCloudIdentity,
  requiresReauthentication,
  useAccountStore,
  type CloudAccountError,
  type CloudAccountView,
  type CloudBootstrapView,
  type CloudMaaSRechargeKeyBalanceView,
} from "../store/accountStore";
import { AccountPayments } from "./AccountPayments";
import { AccountRecharge } from "./AccountRecharge";
import { ModalCloseButton } from "../../../components/ModalCloseButton";
import type { SharedModelCatalogState } from "../../../lib/types";
import { accountBridge, type AccountCloudBindings } from "../bridge/accountBridge";
import { AccountModelShowcase } from "./AccountModelShowcase";

export type { AccountCloudBindings } from "../bridge/accountBridge";

interface AccountCenterProps {
  api?: AccountCloudBindings;
  contextKey?: string;
  request?: AccountCenterRequest;
  avatarSrc?: string;
  modelCatalog?: SharedModelCatalogState;
  onAuthenticated?: (accountScopeKey: string) => void | Promise<void>;
}

export interface AccountCenterRequest {
  id: number;
  page: "overview" | "payments" | "recharge";
  action?: "logout";
}

interface AccountOperationToken {
  epoch: number;
  startScopeKey: string;
}

const SMS_RESEND_COOLDOWN_SECONDS = 60;
const ACCOUNT_AGENT_NETWORK_SRC = new URL("../../../assets/account-agent-network.png", import.meta.url).href;
const BRAND_WORDMARK_SRC = new URL("../../../assets/brand-wordmark.png", import.meta.url).href;
const brandAccountAvatar = new URL("../../../assets/brand-symbol.png", import.meta.url).href;

function formatAccountBalance(value: string | undefined, locale: string): string {
  if (!value) return "—";
  const match = /^(-?)(\d+)(?:\.(\d+))?$/.exec(value.trim());
  if (!match) return value;
  const formatParts = new Intl.NumberFormat(locale).formatToParts(1000.1);
  const groupSeparator = formatParts.find((part) => part.type === "group")?.value ?? ",";
  const decimalSeparator = formatParts.find((part) => part.type === "decimal")?.value ?? ".";
  const integer = match[2].replace(/\B(?=(\d{3})+(?!\d))/g, groupSeparator);
  return `${match[1]}${integer}${match[3] ? `${decimalSeparator}${match[3]}` : ""}`;
}

function formatBillingCredits(value: string | undefined, locale: string): string {
  if (!value) return "—";
  const [integer, fraction = ""] = value.trim().split(".");
  const normalizedFraction = fraction.replace(/0+$/, "");
  return formatAccountBalance(normalizedFraction ? `${integer}.${normalizedFraction}` : integer, locale);
}

interface AccountPasswordFieldProps {
  name: string;
  label: string;
  autoComplete: string;
  disabled: boolean;
  value: string;
  required?: boolean;
  error?: string;
  errorId?: string;
  inputRef?: Ref<HTMLInputElement>;
  onChange(value: string): void;
}

function AccountPasswordField({
  name,
  label,
  autoComplete,
  disabled,
  value,
  required = false,
  error,
  errorId,
  inputRef,
  onChange,
}: AccountPasswordFieldProps) {
  const t = useT();
  const [visible, setVisible] = useState(false);

  useEffect(() => {
    if (!value) setVisible(false);
  }, [value]);

  return (
    <div className="account-center__field">
      <label htmlFor={name}>
        {label}
        {required ? <span className="account-center__required" aria-hidden="true">*</span> : null}
      </label>
      <div className="account-center__password-input">
        <input
          ref={inputRef}
          id={name}
          name={name}
          type={visible ? "text" : "password"}
          autoComplete={autoComplete}
          disabled={disabled}
          required={required}
          aria-invalid={required ? Boolean(error) : undefined}
          aria-describedby={error ? errorId : undefined}
          value={value}
          onChange={(event) => onChange(event.currentTarget.value)}
        />
        <button
          className="account-center__password-toggle"
          type="button"
          disabled={disabled}
          aria-label={t(visible ? "account.hidePassword" : "account.showPassword")}
          aria-pressed={visible}
          title={t(visible ? "account.hidePassword" : "account.showPassword")}
          onMouseDown={(event) => event.preventDefault()}
          onClick={() => setVisible((current) => !current)}
        >
          {visible ? <EyeOff size={16} aria-hidden="true" /> : <Eye size={16} aria-hidden="true" />}
        </button>
      </div>
      {error ? <span className="account-center__field-error" id={errorId}>{error}</span> : null}
    </div>
  );
}

interface RealCaptchaButtonProps {
  api: Pick<AccountCloudBindings, "CloudCaptchaConfig">;
  disabled: boolean;
  title: string;
  label: string;
  action: string;
  hideTitle?: boolean;
  primary?: boolean;
  onBeforeClick?: () => boolean;
  onProof: (proof: AliyunCaptchaProof) => Promise<void>;
  onError: () => void;
}

function RealCaptchaButton({
  api,
  disabled,
  title,
  label,
  action,
  hideTitle = false,
  primary = false,
  onBeforeClick,
  onProof,
  onError,
}: RealCaptchaButtonProps) {
  const elementRef = useRef<HTMLDivElement>(null);
  const buttonRef = useRef<HTMLButtonElement>(null);
  const proofRef = useRef(onProof);
  const errorRef = useRef(onError);
  const [attempt, setAttempt] = useState(0);
  const [status, setStatus] = useState<"loading" | "ready" | "error">("loading");
  proofRef.current = onProof;
  errorRef.current = onError;

  useEffect(() => {
    let active = true;
    let adapter: AliyunCaptchaAdapter | null = null;
    setStatus("loading");
    void api.CloudCaptchaConfig()
      .then(validateAliyunCaptchaConfig)
      .then(async (config) => {
        if (!active || !elementRef.current || !buttonRef.current) return;
        adapter = new AliyunCaptchaAdapter({
          config,
          element: elementRef.current,
          button: buttonRef.current,
          onProof: (proof) => proofRef.current(proof),
          onError: () => errorRef.current(),
        });
        await adapter.init();
        if (active) setStatus("ready");
      })
      .catch(() => {
        adapter?.destroy();
        adapter = null;
        if (!active) return;
        setStatus("error");
        errorRef.current();
      });
    return () => {
      active = false;
      adapter?.destroy();
    };
  }, [api, attempt]);

  return (
    <div
      className={primary ? "account-center__captcha account-center__captcha--primary" : "account-center__captcha"}
      data-aliyun-captcha-status={status}
    >
      {hideTitle ? null : <strong>{title}</strong>}
      <div ref={elementRef} />
      <button
        ref={buttonRef}
        className={primary ? "account-center__primary" : "account-center__button"}
        type="button"
        disabled={disabled || status === "loading"}
        onClickCapture={(event) => {
          if (status !== "ready" || !onBeforeClick || onBeforeClick()) return;
          event.preventDefault();
          event.stopPropagation();
        }}
        onClick={status === "error" ? () => setAttempt((current) => current + 1) : undefined}
        data-account-action={action}
      >
        <ShieldCheck size={14} aria-hidden="true" />
        {label}
      </button>
    </div>
  );
}

function cloudErrorFromMessage(message: string, fallbackCode = "CLOUD_ERROR"): CloudAccountError {
  let displayMessage = message.trim();
  let requestID: string | undefined;
  let diagnosticID: string | undefined;
  for (;;) {
    const metadata = /\s*\[(request_id|diagnostic_id)=([A-Za-z0-9_.:-]{1,128})\]$/.exec(displayMessage);
    if (!metadata) break;
    if (metadata[1] === "request_id") requestID = metadata[2];
    if (metadata[1] === "diagnostic_id" && /^[A-Za-z0-9_-]{1,64}$/.test(metadata[2])) diagnosticID = metadata[2];
    displayMessage = displayMessage.slice(0, metadata.index).trim();
  }
  const match = /^([A-Z][A-Z0-9_]+):\s*(.+)$/.exec(displayMessage);
  if (match) return { code: match[1], message: match[2], request_id: requestID, diagnostic_id: diagnosticID };
  return { code: fallbackCode, message: displayMessage || fallbackCode, request_id: requestID, diagnostic_id: diagnosticID };
}

function cloudError(error: unknown, fallbackMessage: string): CloudAccountError {
  if (error && typeof error === "object") {
    const value = error as { code?: unknown; message?: unknown; request_id?: unknown; diagnostic_id?: unknown };
    const code = typeof value.code === "string" && value.code ? value.code : "";
    const message = typeof value.message === "string" && value.message ? value.message : fallbackMessage;
    const requestID = typeof value.request_id === "string" && value.request_id.trim()
      ? value.request_id.trim()
      : undefined;
    const diagnosticID = typeof value.diagnostic_id === "string" && /^[A-Za-z0-9_-]{1,64}$/.test(value.diagnostic_id.trim())
      ? value.diagnostic_id.trim()
      : undefined;
    if (code) return { code, message, request_id: requestID, diagnostic_id: diagnosticID };
    const parsed = cloudErrorFromMessage(message);
    return { ...parsed, request_id: requestID ?? parsed.request_id, diagnostic_id: diagnosticID ?? parsed.diagnostic_id };
  }
  return cloudErrorFromMessage(String(error || fallbackMessage));
}

function cloudErrorWithoutSensitiveValues(
  error: unknown,
  fallbackMessage: string,
  sensitiveValues: string[],
): CloudAccountError {
  const parsed = cloudError(error, fallbackMessage);
  return sensitiveValues.some((value) => value.length > 0 && parsed.message.includes(value))
    ? { ...parsed, message: fallbackMessage }
    : parsed;
}

export function AccountCenter({
  api = accountBridge,
  contextKey = "default",
  request,
  avatarSrc = "",
  modelCatalog,
  onAuthenticated,
}: AccountCenterProps) {
  const { locale, t } = useI18n();
  const account = useAccountStore((state) => state.account);
  const initialized = useAccountStore((state) => state.initialized);
  const available = useAccountStore((state) => state.available);
  const capabilities = useAccountStore((state) => state.capabilities);
  const balance = useAccountStore((state) => state.balance);
  const subscription = useAccountStore((state) => state.subscription);
  const scopeKey = useAccountStore((state) => state.scopeKey);
  const error = useAccountStore((state) => state.error);
  const busy = useAccountStore((state) => state.busy);
  const activateContext = useAccountStore((state) => state.activateContext);
  const applyBootstrap = useAccountStore((state) => state.applyBootstrap);
  const finishBootstrap = useAccountStore((state) => state.finishBootstrap);
  const setBusy = useAccountStore((state) => state.setBusy);
  const setError = useAccountStore((state) => state.setError);
  const accountKnownMessages = useMemo<Readonly<Record<string, string>>>(() => ({
    INVALID_CREDENTIALS: t("account.error.invalidCredentials"),
    SMS_CODE_INVALID: t("account.error.smsCodeInvalid"),
    SMS_CODE_EXPIRED: t("account.error.smsCodeExpired"),
    SMS_CODE_ATTEMPTS_EXCEEDED: t("account.error.smsCodeAttemptsExceeded"),
    PHONE_INVALID: t("account.error.phoneInvalid"),
    PHONE_ALREADY_REGISTERED: t("account.error.phoneAlreadyRegistered"),
    PASSWORD_INVALID: t("account.error.passwordInvalid"),
    SLIDER_CAPTCHA_REQUIRED: t("account.error.sliderCaptchaRequired"),
    SLIDER_CAPTCHA_EXPIRED: t("account.error.sliderCaptchaExpired"),
    SLIDER_CAPTCHA_REUSED: t("account.error.sliderCaptchaRequired"),
    SMS_RATE_LIMITED: t("account.error.rateLimited"),
    AUTH_RATE_LIMITED: t("account.error.rateLimited"),
    SMS_PROVIDER_UNAVAILABLE: t("account.error.smsProviderUnavailable"),
    NETWORK_UNAVAILABLE: t("account.error.networkUnavailable"),
    ACCOUNT_DISABLED: t("account.error.accountDisabled"),
    AUTH_CRYPTO_CONFIG_UNAVAILABLE: t("account.error.registrationUnavailable"),
    PASSWORD_DECRYPT_FAILED: t("account.error.passwordDecryptFailed"),
  }), [t]);
  const passwordResetKnownMessages = useMemo<Readonly<Record<string, string>>>(() => ({
    ...accountKnownMessages,
    INVALID_CREDENTIALS: t("account.passwordResetRejected"),
    PASSWORD_INVALID: t("account.passwordResetPasswordInvalid"),
    PASSWORD_MISMATCH: t("account.passwordResetPasswordMismatch"),
  }), [accountKnownMessages, t]);
  // A connected cloud always uses the real auth contract. Capability metadata
  // must not downgrade the registration UI to a mock flow.
  const registrationMode = capabilities.registration_mode === "fixture" ? "fixture" : "real";
  const registrationAvailable = available;

  const [mode, setMode] = useState<"login" | "register" | "password-reset">("login");
  const operationKnownMessages = mode === "password-reset"
    ? passwordResetKnownMessages
    : accountKnownMessages;
  const [phone, setPhone] = useState("");
  const [password, setPassword] = useState("");
  const [smsCode, setSMSCode] = useState("");
  const [fieldErrors, setFieldErrors] = useState<Partial<Record<"phone" | "password" | "smsCode", string>>>({});
  const [captchaPassed, setCaptchaPassed] = useState(false);
  const [smsCooldownSeconds, setSMSCooldownSeconds] = useState(0);
  const [resetCodeSent, setResetCodeSent] = useState(false);
  const [resetSMSCode, setResetSMSCode] = useState("");
  const [resetNewPassword, setResetNewPassword] = useState("");
  const [resetConfirmPassword, setResetConfirmPassword] = useState("");
  const [resetFieldErrors, setResetFieldErrors] = useState<Partial<Record<"smsCode" | "newPassword" | "confirmPassword", string>>>({});
  const [resetSMSCooldownSeconds, setResetSMSCooldownSeconds] = useState(0);
  const [passwordResetSucceeded, setPasswordResetSucceeded] = useState(false);
  const requestedPage = request?.page ?? "overview";
  const [page, setPage] = useState<"overview" | "payments" | "recharge">(requestedPage);
  const [rechargeBalance, setRechargeBalance] = useState<CloudMaaSRechargeKeyBalanceView | null>(null);
  const [rechargeBalanceLoading, setRechargeBalanceLoading] = useState(false);
  const [rechargeBalanceUnavailable, setRechargeBalanceUnavailable] = useState(false);
  const [switchStage, setSwitchStage] = useState<"closed" | "credentials" | "confirm">("closed");
  const [switchPhone, setSwitchPhone] = useState("");
  const [switchPassword, setSwitchPassword] = useState("");
  const [switchConfirmPassword, setSwitchConfirmPassword] = useState("");
  const [switchCandidate, setSwitchCandidate] = useState<CloudAccountView | null>(null);

  const operationEpoch = useRef(0);
  const handledLogoutRequestRef = useRef(0);
  const actionMenuRef = useRef<HTMLDetailsElement>(null);
  const phoneInputRef = useRef<HTMLInputElement>(null);
  const passwordInputRef = useRef<HTMLInputElement>(null);
  const smsCodeInputRef = useRef<HTMLInputElement>(null);
  const resetSMSCodeInputRef = useRef<HTMLInputElement>(null);
  const resetNewPasswordInputRef = useRef<HTMLInputElement>(null);
  const resetConfirmPasswordInputRef = useRef<HTMLInputElement>(null);
  const clearPasswordResetState = useCallback(() => {
    setResetCodeSent(false);
    setResetSMSCode("");
    setResetNewPassword("");
    setResetConfirmPassword("");
    setResetFieldErrors({});
    setResetSMSCooldownSeconds(0);
  }, []);
  const clearSwitchState = useCallback(() => {
    setSwitchStage("closed");
    setSwitchPhone("");
    setSwitchPassword("");
    setSwitchConfirmPassword("");
    setSwitchCandidate(null);
  }, []);
  const isCurrentOperation = useCallback((token: AccountOperationToken) => operationEpoch.current === token.epoch, []);
  const isCurrentScopedOperation = useCallback((token: AccountOperationToken) => (
    operationEpoch.current === token.epoch
    && useAccountStore.getState().scopeKey === token.startScopeKey
  ), []);
  const beginOperation = useCallback(() => {
    const epoch = operationEpoch.current + 1;
    operationEpoch.current = epoch;
    setBusy(true);
    setError(null);
    return { epoch, startScopeKey: useAccountStore.getState().scopeKey };
  }, [setBusy, setError]);
  const commitView = useCallback((token: AccountOperationToken, view: CloudBootstrapView) => {
    if (isCurrentOperation(token)) applyBootstrap(view);
  }, [applyBootstrap, isCurrentOperation]);
  const commitAccountError = useCallback((token: AccountOperationToken, nextError: CloudAccountError) => {
    if (!isCurrentScopedOperation(token)) return;
    const presentation = reportClientError("account.operation", nextError, {
      fallbackMessage: t("account.requestFailed"),
      knownMessages: operationKnownMessages,
    });
    setError({ ...nextError, message: presentation.userMessage });
  }, [isCurrentScopedOperation, operationKnownMessages, setError, t]);
  const commitError = useCallback((token: AccountOperationToken, cause: unknown) => {
    commitAccountError(token, cloudError(cause, t("account.requestFailed")));
  }, [commitAccountError, t]);
  const finishOperation = useCallback((token: AccountOperationToken) => {
    if (isCurrentOperation(token)) setBusy(false);
  }, [isCurrentOperation, setBusy]);
  const notifyAuthenticated = useCallback(async (
    token: AccountOperationToken,
    view: CloudBootstrapView,
  ) => {
    if (view.account.status !== "authenticated" || !isCurrentOperation(token)) return;
    try {
      await onAuthenticated?.(cloudScopeKey(view.account));
    } catch {
      // The authenticated account state must not be rolled back by a downstream refresh failure.
    }
  }, [isCurrentOperation, onAuthenticated]);

  const validatePhone = () => {
    if (phone.trim()) {
      setFieldErrors((current) => current.phone ? { ...current, phone: undefined } : current);
      return true;
    }
    setFieldErrors((current) => ({ ...current, phone: t("account.phoneRequired") }));
    phoneInputRef.current?.focus();
    return false;
  };

  const validateAuthentication = () => {
    const nextErrors: Partial<Record<"phone" | "password" | "smsCode", string>> = {};
    if (!phone.trim()) nextErrors.phone = t("account.phoneRequired");
    if (!password) nextErrors.password = t("account.passwordRequired");
    if (mode === "register" && !smsCode.trim()) nextErrors.smsCode = t("account.smsCodeRequired");
    setFieldErrors(nextErrors);
    if (nextErrors.phone) phoneInputRef.current?.focus();
    else if (nextErrors.password) passwordInputRef.current?.focus();
    else if (nextErrors.smsCode) smsCodeInputRef.current?.focus();
    return Object.keys(nextErrors).length === 0;
  };

  const switchAuthenticationMode = (nextMode: "login" | "register") => {
    if (nextMode === mode) return;
    setMode(nextMode);
    setError(null);
    setFieldErrors({});
    setCaptchaPassed(false);
  };

  useLayoutEffect(() => {
    operationEpoch.current += 1;
    activateContext(contextKey);
    setMode("login");
    setPhone("");
    setPassword("");
    setSMSCode("");
    setFieldErrors({});
    setCaptchaPassed(false);
    setSMSCooldownSeconds(0);
    clearPasswordResetState();
    setPasswordResetSucceeded(false);
    clearSwitchState();
    return () => {
      operationEpoch.current += 1;
    };
  }, [activateContext, clearPasswordResetState, clearSwitchState, contextKey]);

  useLayoutEffect(() => {
    setPage(requestedPage);
    setCaptchaPassed(false);
    clearPasswordResetState();
    setPasswordResetSucceeded(false);
    clearSwitchState();
  }, [clearPasswordResetState, clearSwitchState, contextKey, request?.id, requestedPage, scopeKey]);

  useLayoutEffect(() => {
    if (registrationAvailable) return;
    setMode("login");
    setCaptchaPassed(false);
    setSMSCooldownSeconds(0);
  }, [registrationAvailable]);

  useEffect(() => {
    if (smsCooldownSeconds <= 0) return;
    const timer = window.setTimeout(() => {
      setSMSCooldownSeconds((current) => Math.max(0, current - 1));
    }, 1000);
    return () => window.clearTimeout(timer);
  }, [smsCooldownSeconds]);

  useEffect(() => {
    if (resetSMSCooldownSeconds <= 0) return;
    const timer = window.setTimeout(() => {
      setResetSMSCooldownSeconds((current) => Math.max(0, current - 1));
    }, 1000);
    return () => window.clearTimeout(timer);
  }, [resetSMSCooldownSeconds]);

  useEffect(() => {
    const epoch = beginOperation();
    void api.CloudBootstrap()
      .then((view) => {
        commitView(epoch, view);
      })
      .catch((cause) => {
        commitError(epoch, cause);
      })
      .finally(() => {
        if (isCurrentOperation(epoch)) finishBootstrap();
      });
    return () => {
      operationEpoch.current += 1;
    };
  }, [api, beginOperation, commitError, commitView, contextKey, finishBootstrap, isCurrentOperation]);

  const sendSMS = async (proof?: AliyunCaptchaProof) => {
    if (!validatePhone()) return;
    const realProofReady = registrationMode === "real" && proof !== undefined;
    if (!registrationAvailable || (!realProofReady && !captchaPassed) || smsCooldownSeconds > 0) {
      if (!registrationAvailable) setMode("login");
      setCaptchaPassed(false);
      return;
    }
    const epoch = beginOperation();
    try {
      await api.CloudSendRegisterSMS(registrationMode === "real"
        ? { phone, captcha_ticket: proof?.ticket, captcha_provider: proof?.provider }
        : { phone, captcha_passed: captchaPassed });
      if (isCurrentOperation(epoch)) setSMSCooldownSeconds(SMS_RESEND_COOLDOWN_SECONDS);
    } catch (cause) {
      commitAccountError(epoch, cloudErrorWithoutSensitiveValues(cause, t("account.requestFailed"), [phone]));
    } finally {
      if (isCurrentOperation(epoch)) setCaptchaPassed(false);
      finishOperation(epoch);
    }
  };

  const openPasswordReset = () => {
    setError(null);
    setPassword("");
    setPasswordResetSucceeded(false);
    clearPasswordResetState();
    setMode("password-reset");
  };

  const closePasswordReset = () => {
    setError(null);
    clearPasswordResetState();
    setMode("login");
  };

  const sendPasswordResetSMS = async (proof?: AliyunCaptchaProof) => {
    if (!proof || !phone.trim() || resetSMSCooldownSeconds > 0) return;
    const token = beginOperation();
    try {
      await api.CloudSendPasswordResetSMS({
        phone,
        captcha_ticket: proof.ticket,
        captcha_provider: proof.provider,
      });
      if (isCurrentOperation(token)) {
        setResetCodeSent(true);
        setResetSMSCooldownSeconds(SMS_RESEND_COOLDOWN_SECONDS);
      }
    } catch (cause) {
      commitAccountError(token, cloudErrorWithoutSensitiveValues(cause, t("account.requestFailed"), [phone]));
    } finally {
      finishOperation(token);
    }
  };

  const resetPassword = async () => {
    const nextErrors: Partial<Record<"smsCode" | "newPassword" | "confirmPassword", string>> = {};
    if (!resetSMSCode.trim()) nextErrors.smsCode = t("account.smsCodeRequired");
    if (!resetNewPassword) nextErrors.newPassword = t("account.passwordResetNewPasswordRequired");
    if (!resetConfirmPassword) nextErrors.confirmPassword = t("account.passwordResetConfirmPasswordRequired");
    setResetFieldErrors(nextErrors);
    if (nextErrors.smsCode) resetSMSCodeInputRef.current?.focus();
    else if (nextErrors.newPassword) resetNewPasswordInputRef.current?.focus();
    else if (nextErrors.confirmPassword) resetConfirmPasswordInputRef.current?.focus();
    if (Object.keys(nextErrors).length > 0) return;

    const passwordLength = Array.from(resetNewPassword).length;
    if (passwordLength < 8 || passwordLength > 20) {
      setError({ code: "PASSWORD_INVALID", message: t("account.passwordResetPasswordInvalid") });
      return;
    }
    if (resetNewPassword !== resetConfirmPassword) {
      setError({ code: "PASSWORD_MISMATCH", message: t("account.passwordResetPasswordMismatch") });
      return;
    }
    if (!/^[0-9]{6}$/.test(resetSMSCode)) {
      setError({ code: "SMS_CODE_INVALID", message: t("account.passwordResetCodeInvalid") });
      return;
    }
    const request: CloudResetPasswordRequest = {
      phone,
      new_password: resetNewPassword,
      sms_code: resetSMSCode,
    };
    const token = beginOperation();
    try {
      await api.CloudResetPassword(request);
      if (!isCurrentOperation(token)) return;
      clearPasswordResetState();
      setMode("login");
      setPasswordResetSucceeded(true);
    } catch (cause) {
      commitAccountError(token, cloudErrorWithoutSensitiveValues(
        cause,
        t("account.requestFailed"),
        [request.phone, request.new_password, request.sms_code, resetConfirmPassword],
      ));
    } finally {
      if (isCurrentOperation(token)) {
        setResetNewPassword("");
        setResetConfirmPassword("");
      }
      finishOperation(token);
    }
  };

  const stageSwitchCandidate = (token: AccountOperationToken, view: CloudBootstrapView, requestPhone: string): boolean => {
    const candidate = view.operation_error?.code === "PROFILE_SCOPE_MISMATCH" ? view.candidate ?? null : null;
    const candidateIsDifferent = Boolean(
      candidate?.cloud_user_id
      && candidate.cloud_user_id !== view.account.cloud_user_id,
    );
    if (!candidate || !candidateIsDifferent || cloudScopeKey(view.account) !== token.startScopeKey) return false;
    commitView(token, { ...view, operation_error: null, candidate: null });
    if (!isCurrentScopedOperation(token)) return true;
    setSwitchPhone(requestPhone);
    setSwitchPassword("");
    setSwitchConfirmPassword("");
    setSwitchCandidate(candidate);
    setSwitchStage("confirm");
    return true;
  };

  const login = async (proof?: AliyunCaptchaProof) => {
    if (!validateAuthentication()) return;
    if (registrationMode === "real" && !proof) return;
    const request: CloudLoginRequest = {
      phone,
      password,
      ...(registrationMode === "real" ? { captcha_ticket: proof?.ticket, captcha_provider: proof?.provider } : {}),
    };
    const token = beginOperation();
    try {
      const view = await api.CloudLogin(request);
      if (!isCurrentOperation(token)) return;
      if (!stageSwitchCandidate(token, view, request.phone)) {
        commitView(token, view);
        await notifyAuthenticated(token, view);
      }
    } catch (cause) {
      commitAccountError(token, cloudErrorWithoutSensitiveValues(cause, t("account.requestFailed"), [request.phone, request.password]));
    } finally {
      if (isCurrentOperation(token)) {
        setPassword("");
      }
      finishOperation(token);
    }
  };

  const register = async () => {
    if (!registrationAvailable) {
      setMode("login");
      setCaptchaPassed(false);
      return;
    }
    if (!validateAuthentication()) return;
    const request: CloudRegisterRequest = { phone, password, sms_code: smsCode };
    const token = beginOperation();
    try {
      const view = await api.CloudRegister(request);
      if (!isCurrentOperation(token)) return;
      if (!stageSwitchCandidate(token, view, request.phone)) {
        commitView(token, view);
        await notifyAuthenticated(token, view);
      }
      if (isCurrentOperation(token)) setSMSCode("");
    } catch (cause) {
      commitAccountError(token, cloudErrorWithoutSensitiveValues(
        cause,
        t("account.requestFailed"),
        [request.phone, request.password, request.sms_code],
      ));
    } finally {
      if (isCurrentOperation(token)) setPassword("");
      finishOperation(token);
    }
  };

  const logout = useCallback(async () => {
    const epoch = beginOperation();
    try {
      commitView(epoch, await api.CloudLogout());
    } catch (cause) {
      commitError(epoch, cause);
    } finally {
      finishOperation(epoch);
    }
  }, [api, beginOperation, commitError, commitView, finishOperation]);

  const refresh = async () => {
    const epoch = beginOperation();
    try {
      const view = await api.CloudRefreshAssets();
      commitView(epoch, view);
      await notifyAuthenticated(epoch, view);
    } catch (cause) {
      commitError(epoch, cause);
    } finally {
      finishOperation(epoch);
    }
  };

  const openSwitch = () => {
    setError(null);
    setPage("overview");
    setSwitchStage("credentials");
    setSwitchPhone("");
    setSwitchPassword("");
    setSwitchConfirmPassword("");
    setSwitchCandidate(null);
  };

  const cancelSwitch = () => {
    operationEpoch.current += 1;
    setBusy(false);
    setError(null);
    clearSwitchState();
  };

  const verifySwitchCandidate = async (proof?: AliyunCaptchaProof) => {
    if (!switchPhone.trim() || !switchPassword || (registrationMode === "real" && !proof)) return;
    const request: CloudLoginRequest = {
      phone: switchPhone,
      password: switchPassword,
      ...(registrationMode === "real" ? { captcha_ticket: proof?.ticket, captcha_provider: proof?.provider } : {}),
    };
    setSwitchPassword("");
    const token = beginOperation();
    try {
      const view = await api.CloudLogin(request);
      if (!isCurrentOperation(token)) return;
      if (stageSwitchCandidate(token, view, request.phone)) return;
      if (cloudScopeKey(view.account) === token.startScopeKey) commitView(token, view);
      commitAccountError(token, {
        code: "ACCOUNT_SWITCH_DIFFERENT_REQUIRED",
        message: t("account.switchDifferentRequired"),
      });
    } catch (cause) {
      commitAccountError(token, cloudErrorWithoutSensitiveValues(cause, t("account.requestFailed"), [request.phone, request.password]));
    } finally {
      finishOperation(token);
    }
  };

  const confirmSwitch = async (proof?: AliyunCaptchaProof) => {
    if (!switchCandidate || !switchPhone.trim() || !switchConfirmPassword || (registrationMode === "real" && !proof)) return;
    const request: CloudLoginRequest = {
      phone: switchPhone,
      password: switchConfirmPassword,
      ...(registrationMode === "real" ? { captcha_ticket: proof?.ticket, captcha_provider: proof?.provider } : {}),
    };
    setSwitchConfirmPassword("");
    const token = beginOperation();
    try {
      const view = await api.CloudSwitchAccount(request);
      commitView(token, view);
      await notifyAuthenticated(token, view);
      if (isCurrentOperation(token)) clearSwitchState();
    } catch (cause) {
      const parsed = cloudErrorWithoutSensitiveValues(cause, t("account.requestFailed"), [request.phone, request.password]);
      commitAccountError(token, parsed.code === "PROFILE_SCOPE_MISMATCH"
        ? { ...parsed, message: t("account.switchStateChanged") }
        : parsed);
    } finally {
      finishOperation(token);
    }
  };

  const reportCaptchaError = () => {
    setError({ code: "SLIDER_CAPTCHA_REQUIRED", message: t("account.requestFailed") });
  };

  const hasRecoverableAccount = hasRecoverableCloudIdentity(account);
  const reauthenticationRequired = requiresReauthentication(account);
  const unavailable = !available || error?.code === "CLOUD_UNAVAILABLE";
  const identityLabel = account?.display_name?.trim() || t("account.nameUnavailable");
  const currentPlan = subscription?.plans.find((plan) => plan.plan === subscription.current.plan);
  const currentPlanLabel = subscription?.current.plan === "pro"
    ? "体验套餐"
    : currentPlan?.display_name
    || (subscription?.current.plan === "free" ? t("account.plan.free") : subscription?.current.plan)
    || "—";
  const profileMismatch = error?.code === "PROFILE_SCOPE_MISMATCH";
  const visibleErrorMessage = error
    ? clientErrorPresentation(error, {
      fallbackMessage: t("account.requestFailed"),
      knownMessages: operationKnownMessages,
    }).userMessage
    : "";

  useEffect(() => {
    if (request?.action !== "logout" || !initialized || !hasRecoverableAccount) return;
    if (handledLogoutRequestRef.current === request.id) return;
    handledLogoutRequestRef.current = request.id;
    void logout();
  }, [hasRecoverableAccount, initialized, logout, request?.action, request?.id]);

  useEffect(() => {
    if (page === "overview") return;
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape") setPage("overview");
    };
    document.addEventListener("keydown", closeOnEscape);
    return () => document.removeEventListener("keydown", closeOnEscape);
  }, [page]);

  useEffect(() => {
    if (page === "overview" || account?.status !== "authenticated") return;
    let active = true;
    setRechargeBalanceUnavailable(false);
    setRechargeBalanceLoading(true);
    void api.CloudGetMaaSRechargeKeyBalance()
      .then((view) => {
        if (active && useAccountStore.getState().scopeKey === scopeKey) setRechargeBalance(view);
      })
      .catch((cause: unknown) => {
        if (active) {
          reportClientError("account.maasBalance", cause, { fallbackMessage: t("account.billing.balanceUnavailable") });
          setRechargeBalance(null);
          setRechargeBalanceUnavailable(true);
        }
      })
      .finally(() => {
        if (active) setRechargeBalanceLoading(false);
      });
    return () => {
      active = false;
    };
  }, [account?.status, api, page, scopeKey, t]);

  const switchSection = switchStage !== "closed" ? (
    <section className="account-center__switch" data-account-switch-stage={switchStage}>
      <div className="account-center__switch-heading">
        <div>
          <h2>{t("account.switchTitle")}</h2>
          <p>{t(switchStage === "credentials" ? "account.switchCredentialsHint" : "account.switchConfirmHint")}</p>
        </div>
        <ModalCloseButton
          className="account-center__switch-close"
          label={t("common.cancel")}
          disabled={busy}
          onClick={cancelSwitch}
          data-account-action="switch-cancel"
        />
      </div>
      {switchStage === "credentials" ? (
        <div className="account-center__switch-form">
          <label className="account-center__field">
            <span>{t("account.phone")}</span>
            <input
              name="switch_phone"
              inputMode="numeric"
              autoComplete="tel"
              disabled={busy}
              value={switchPhone}
              onChange={(event) => setSwitchPhone(event.currentTarget.value)}
            />
          </label>
          <AccountPasswordField
            name="switch_password"
            label={t("account.password")}
            autoComplete="current-password"
            disabled={busy}
            value={switchPassword}
            onChange={setSwitchPassword}
          />
          {registrationMode === "real" ? (
            <RealCaptchaButton
              api={api}
              disabled={busy || !switchPhone.trim() || !switchPassword}
              title={t("account.passCaptcha")}
              label={busy ? t("common.loading") : t("account.switchVerify")}
              action="switch-verify"
              onProof={verifySwitchCandidate}
              onError={reportCaptchaError}
            />
          ) : (
            <button
              className="account-center__primary"
              type="button"
              disabled={busy || !switchPhone.trim() || !switchPassword}
              onClick={() => void verifySwitchCandidate()}
              data-account-action="switch-verify"
            >
              {busy ? t("common.loading") : t("account.switchVerify")}
            </button>
          )}
        </div>
      ) : switchCandidate ? (
        <div className="account-center__switch-form">
          <div className="account-center__switch-identities">
            <div>
              <span>{t("account.switchCurrent")}</span>
              <strong>{identityLabel}</strong>
            </div>
            <ArrowRightLeft size={18} aria-hidden="true" />
            <div data-account-switch-candidate={switchCandidate.cloud_user_id}>
              <span>{t("account.switchTarget")}</span>
              <strong>{switchCandidate.display_name || switchCandidate.cloud_user_id || "—"}</strong>
            </div>
          </div>
          <p className="account-center__switch-warning">{t("account.switchImpactWarning")}</p>
          <AccountPasswordField
            name="switch_confirm_password"
            label={t("account.switchConfirmPassword")}
            autoComplete="current-password"
            disabled={busy}
            value={switchConfirmPassword}
            onChange={setSwitchConfirmPassword}
          />
          {registrationMode === "real" ? (
            <RealCaptchaButton
              api={api}
              disabled={busy || !switchConfirmPassword}
              title={t("account.passCaptcha")}
              label={busy ? t("common.loading") : t("account.switchConfirm")}
              action="switch-confirm"
              onProof={confirmSwitch}
              onError={reportCaptchaError}
            />
          ) : (
            <button
              className="account-center__primary"
              type="button"
              disabled={busy || !switchConfirmPassword}
              onClick={() => void confirmSwitch()}
              data-account-action="switch-confirm"
            >
              {busy ? t("common.loading") : t("account.switchConfirm")}
            </button>
          )}
        </div>
      ) : null}
    </section>
  ) : null;

  const showAnonymousAuth = initialized && !unavailable && !hasRecoverableAccount && switchStage === "closed";
  const accountCenterClassName = showAnonymousAuth
    ? "account-center account-center--auth"
    : hasRecoverableAccount
      ? "account-center account-center--authenticated"
      : "account-center";

  return (
    <section
      className={accountCenterClassName}
      data-account-state={account?.status ?? "loading"}
    >
      {showAnonymousAuth ? null : (
        hasRecoverableAccount ? (
          <header className="account-center__account-bar-wrap">
            <div className="account-center__account-bar">
              <div className="account-center__account-identity">
                <img src={avatarSrc || brandAccountAvatar} alt="" />
                <span>
                  <strong data-account-identity>{identityLabel}</strong>
                </span>
              </div>
              <div className="account-center__account-metric account-center__account-metric--balance">
                <small>余额</small>
                <strong data-account-balance>{formatAccountBalance(balance?.total_balance, locale)}</strong>
              </div>
              <div className="account-center__account-metric account-center__account-metric--plan">
                <small>套餐</small>
                <strong data-account-current-plan>{currentPlanLabel}</strong>
              </div>
              <button
                className="account-center__account-icon-button"
                type="button"
                aria-label={t("account.refresh")}
                title={t("account.refresh")}
                disabled={busy || switchStage !== "closed"}
                onClick={() => void refresh()}
                data-account-action="refresh"
              >
                <RefreshCw size={17} aria-hidden="true" />
              </button>
              <button
                className="account-center__account-purchase"
                type="button"
                data-account-page="payments"
                data-capability-unavailable={!capabilities.payments ? "true" : undefined}
                disabled={busy || switchStage !== "closed"}
                onClick={() => setPage("payments")}
              >
                积分与套餐
              </button>
              <details className="account-center__action-menu account-center__action-menu--showcase" ref={actionMenuRef}>
                <summary className="account-center__account-menu-trigger">
                  <UserRound size={15} aria-hidden="true" />
                  {t("account.actions")}
                  <ChevronDown size={14} aria-hidden="true" />
                </summary>
                <div className="account-center__action-menu-panel">
                  <div className="account-center__action-menu-profile">
                    <img src={avatarSrc || brandAccountAvatar} alt="" />
                    <span>
                      <strong>{identityLabel}</strong>
                      <small>当前账户 · {currentPlanLabel}</small>
                    </span>
                    <i aria-label={t("account.status.authenticated")} />
                  </div>
                  <button
                    type="button"
                    disabled={busy || switchStage !== "closed"}
                    onClick={() => {
                      if (actionMenuRef.current) actionMenuRef.current.open = false;
                      openSwitch();
                    }}
                    data-account-action="switch-open"
                  >
                    <ArrowRightLeft size={15} aria-hidden="true" />
                    <span><strong>{t("account.switchOpen")}</strong><small>使用其他云端账户</small></span>
                  </button>
                  <button
                    className="account-center__action-menu-danger"
                    type="button"
                    disabled={busy || switchStage !== "closed"}
                    onClick={() => {
                      if (actionMenuRef.current) actionMenuRef.current.open = false;
                      void logout();
                    }}
                    data-account-action="logout"
                  >
                    <LogOut size={15} aria-hidden="true" />
                    <span><strong>{t("account.logout")}</strong><small>退出当前云端账户</small></span>
                  </button>
                </div>
              </details>
            </div>
          </header>
        ) : (
          <header className="account-center__header">
            <div className="account-center__heading">
              <div className="account-center__title-icon" aria-hidden="true"><UserRound size={20} /></div>
              <div><h1>{t("account.title")}</h1><p>{t("account.subtitle")}</p></div>
            </div>
          </header>
        )
      )}

      {error ? (
        <div className="account-center__error" role="alert">
          <span className="account-center__error-icon" aria-hidden="true">
            <CircleAlert size={17} />
          </span>
          <div className="account-center__error-content">
            <span className="account-center__error-message">{profileMismatch
              ? (switchCandidate ? t("account.switchStateChanged") : t("account.profileMismatchRecovery"))
              : visibleErrorMessage}</span>
            {!profileMismatch && (error.code || error.request_id || error.diagnostic_id) ? (
              <details className="account-center__error-details">
                <summary>{t("account.technicalDetails")}</summary>
                {error.code ? (
                  <strong className="account-center__error-code" data-account-error-code translate="no">
                    {t("account.errorCode", { code: error.code })}
                  </strong>
                ) : null}
                {error.request_id ? (
                  <small data-account-error-request-id translate="no">
                    {t("account.requestID", { requestID: error.request_id })}
                  </small>
                ) : null}
                {error.diagnostic_id ? (
                  <small className="account-center__error-log-id" data-account-error-diagnostic-id translate="no">
                    <span>{t("account.logID")}</span>
                    <code>{error.diagnostic_id}</code>
                    <CopyButton text={error.diagnostic_id} showInlineLabel={false} label={t("common.copy")} />
                  </small>
                ) : null}
              </details>
            ) : null}
          </div>
        </div>
      ) : null}
      {!initialized ? (
        <div className="account-center__unavailable" data-account-loading>
          <RefreshCw size={18} aria-hidden="true" />
          <p>{t("common.loading")}</p>
        </div>
      ) : unavailable && !hasRecoverableAccount ? (
        <div className="account-center__unavailable">
          <ShieldCheck size={18} aria-hidden="true" />
          <p>{t("account.unavailable")}</p>
        </div>
      ) : hasRecoverableAccount ? (
        <div className="account-center__authenticated">
          {page === "overview" ? (
            <div className="account-center__overview account-center__overview--showcase" data-account-page-panel="overview">
              <AccountModelShowcase modelCatalog={modelCatalog} />
            </div>
          ) : (
            <div
              className="account-center__payment-page account-subscription-backdrop"
              data-account-page-panel={page}
              onMouseDown={(event) => {
                if (event.target === event.currentTarget) setPage("overview");
              }}
            >
              <section
                aria-labelledby="account-billing-title"
                aria-modal="true"
                className="account-subscription-modal account-billing-modal"
                data-subscription-modal="true"
                role="dialog"
              >
                <header className="account-subscription-modal__header">
                  <div className="account-billing-modal__heading">
                    <span className="account-billing-modal__mark" aria-hidden="true"><Coins size={22} /></span>
                    <div>
                      <h2 id="account-billing-title">{t("account.billing.modalTitle")}</h2>
                      <p>{t("account.billing.modalSubtitle")}</p>
                    </div>
                  </div>
                  <div className="account-billing-modal__header-actions">
                    <div className="account-billing-modal__balance account-billing-modal__balance--subscription">
                      <span><i aria-hidden="true" />{t("account.billing.subscriptionCredits")}</span>
                      <strong>{rechargeBalanceLoading
                        ? t("common.loading")
                        : rechargeBalanceUnavailable
                          ? t("account.billing.balanceUnavailable")
                          : formatBillingCredits(rechargeBalance?.package_available_credits, locale)}</strong>
                    </div>
                    <div className="account-billing-modal__balance account-billing-modal__balance--recharge">
                      <span><i aria-hidden="true" />{t("account.billing.rechargeCredits")}</span>
                      <strong>{rechargeBalanceLoading
                        ? t("common.loading")
                        : rechargeBalanceUnavailable
                          ? t("account.billing.balanceUnavailable")
                          : formatBillingCredits(rechargeBalance?.available_credits, locale)}</strong>
                    </div>
                    <ModalCloseButton label={t("common.close")} onClick={() => setPage("overview")} />
                  </div>
                </header>
                <div className="account-billing-modal__tabs" role="tablist" aria-label={t("account.billing.modalTitle")}>
                  <button
                    aria-selected={page === "payments"}
                    className={page === "payments" ? "is-active" : ""}
                    onClick={() => setPage("payments")}
                    role="tab"
                    type="button"
                  >
                    {t("account.billing.plansTab")}
                  </button>
                  <button
                    aria-selected={page === "recharge"}
                    className={page === "recharge" ? "is-active" : ""}
                    onClick={() => setPage("recharge")}
                    role="tab"
                    type="button"
                  >
                    {t("account.billing.rechargeTab")}
                  </button>
                </div>
                <div className="account-subscription-modal__scroll">
                  {page === "payments" ? (
                    <AccountPayments
                      key={scopeKey}
                      api={api}
                      paymentsEnabled={capabilities.payments}
                      scopeKey={scopeKey}
                    />
                  ) : (
                    <AccountRecharge
                      key={scopeKey}
                      api={api}
                      balance={rechargeBalance}
                      onBalanceChange={setRechargeBalance}
                      scopeKey={scopeKey}
                    />
                  )}
                </div>
              </section>
            </div>
          )}
        </div>
      ) : (
        <div className="account-center__auth">
          <section className="account-center__auth-shell" aria-labelledby="account-auth-title">
            <div className="account-center__auth-story">
              <img
                className="account-center__auth-wordmark"
                src={BRAND_WORDMARK_SRC}
                alt={BRAND_DISPLAY_NAME}
                draggable={false}
              />
              <h2>{t("account.agentHeadline")}</h2>
              <p>{t("account.agentSubtitle")}</p>
              <img
                className="account-center__auth-illustration"
                src={ACCOUNT_AGENT_NETWORK_SRC}
                alt=""
                aria-hidden="true"
                draggable={false}
              />
            </div>
            <div className="account-center__auth-panel">
              <div className="account-center__auth-form">
                <header className="account-center__auth-heading">
                  <h1 id="account-auth-title">
                    {t(mode === "login"
                      ? "account.loginTitle"
                      : mode === "register"
                        ? "account.registerTitle"
                        : "account.passwordResetTitle")}
                  </h1>
                  <p>{t(mode === "login"
                    ? "account.loginSubtitle"
                    : mode === "register"
                      ? "account.registerSubtitle"
                      : "account.passwordResetSubtitle")}</p>
                </header>
                {reauthenticationRequired ? (
                  <p className="account-center__auth-hint" data-account-reauthentication>
                    <CircleAlert size={15} aria-hidden="true" />
                    {t("account.reauthenticationRequired")}
                  </p>
                ) : null}
                {passwordResetSucceeded && mode === "login" ? (
                  <p className="account-center__auth-success" role="status" data-account-password-reset-success>
                    <ShieldCheck size={16} aria-hidden="true" />
                    {t("account.passwordResetSucceeded")}
                  </p>
                ) : null}
                <div className="account-center__auth-card">
                  {mode === "password-reset" ? (
                    <button
                      className="account-center__auth-back"
                      type="button"
                      disabled={busy}
                      onClick={closePasswordReset}
                      data-account-action="password-reset-back"
                    >
                      <ArrowLeft size={15} aria-hidden="true" />
                      {t("account.passwordResetBack")}
                    </button>
                  ) : (
                    <div className="account-center__modes">
                      <button
                        className={mode === "login" ? "account-center__mode account-center__mode--active" : "account-center__mode"}
                        type="button"
                        disabled={busy}
                        aria-pressed={mode === "login"}
                        onClick={() => {
                          setPasswordResetSucceeded(false);
                          switchAuthenticationMode("login");
                        }}
                        data-account-mode="login"
                      >
                        {t("account.loginTab")}
                      </button>
                      <button
                        className={mode === "register" ? "account-center__mode account-center__mode--active" : "account-center__mode"}
                        type="button"
                        disabled={busy || !registrationAvailable}
                        aria-pressed={mode === "register"}
                        onClick={() => {
                          setPasswordResetSucceeded(false);
                          if (!registrationAvailable) {
                            setMode("login");
                            setCaptchaPassed(false);
                            return;
                          }
                          switchAuthenticationMode("register");
                        }}
                        data-account-mode="register"
                        data-capability-unavailable={!registrationAvailable ? "true" : undefined}
                      >
                        {t("account.registerTab")}
                        {!registrationAvailable ? <span className="account-center__coming-soon">{t("account.comingLater")}</span> : null}
                      </button>
                    </div>
                  )}
                  <label className="account-center__field">
                    <span>
                      {t("account.phone")}
                      <span className="account-center__required" aria-hidden="true">*</span>
                    </span>
                    <input
                      ref={phoneInputRef}
                      name="phone"
                      inputMode="numeric"
                      autoComplete="tel"
                      disabled={busy || (mode === "password-reset" && resetCodeSent)}
                      required
                      aria-invalid={Boolean(fieldErrors.phone)}
                      aria-describedby={fieldErrors.phone ? "account-phone-error" : undefined}
                      value={phone}
                      onChange={(event) => {
                        const value = event.currentTarget.value;
                        setPhone(value);
                        if (value.trim()) {
                          setFieldErrors((current) => current.phone ? { ...current, phone: undefined } : current);
                        }
                      }}
                    />
                    {fieldErrors.phone
                      ? <span className="account-center__field-error" id="account-phone-error">{fieldErrors.phone}</span>
                      : null}
                  </label>
                  {mode === "password-reset" ? (
                    resetCodeSent ? (
                      <>
                        <p className="account-center__reset-sent" role="status">
                          {t("account.passwordResetCodeSent", { phone })}
                        </p>
                        <label className="account-center__field">
                          <span>
                            {t("account.smsCode")}
                            <span className="account-center__required" aria-hidden="true">*</span>
                          </span>
                          <input
                            ref={resetSMSCodeInputRef}
                            name="password_reset_sms_code"
                            inputMode="numeric"
                            autoComplete="one-time-code"
                            disabled={busy}
                            required
                            aria-invalid={Boolean(resetFieldErrors.smsCode)}
                            aria-describedby={resetFieldErrors.smsCode ? "account-password-reset-sms-code-error" : undefined}
                            value={resetSMSCode}
                            onChange={(event) => {
                              const value = event.currentTarget.value;
                              setResetSMSCode(value);
                              if (value.trim()) {
                                setResetFieldErrors((current) => current.smsCode ? { ...current, smsCode: undefined } : current);
                              }
                            }}
                          />
                          {resetFieldErrors.smsCode
                            ? <span className="account-center__field-error" id="account-password-reset-sms-code-error">{resetFieldErrors.smsCode}</span>
                            : null}
                        </label>
                        <AccountPasswordField
                          name="password_reset_new_password"
                          label={t("account.passwordResetNewPassword")}
                          autoComplete="new-password"
                          disabled={busy}
                          value={resetNewPassword}
                          required
                          error={resetFieldErrors.newPassword}
                          errorId="account-password-reset-new-password-error"
                          inputRef={resetNewPasswordInputRef}
                          onChange={(value) => {
                            setResetNewPassword(value);
                            if (value) {
                              setResetFieldErrors((current) => current.newPassword ? { ...current, newPassword: undefined } : current);
                            }
                          }}
                        />
                        <AccountPasswordField
                          name="password_reset_confirm_password"
                          label={t("account.passwordResetConfirmPassword")}
                          autoComplete="new-password"
                          disabled={busy}
                          value={resetConfirmPassword}
                          required
                          error={resetFieldErrors.confirmPassword}
                          errorId="account-password-reset-confirm-password-error"
                          inputRef={resetConfirmPasswordInputRef}
                          onChange={(value) => {
                            setResetConfirmPassword(value);
                            if (value) {
                              setResetFieldErrors((current) => current.confirmPassword ? { ...current, confirmPassword: undefined } : current);
                            }
                          }}
                        />
                        <button
                          className="account-center__primary"
                          type="button"
                          disabled={busy}
                          onClick={() => void resetPassword()}
                          data-account-action="password-reset-submit"
                        >
                          {busy ? t("common.loading") : t("account.passwordResetSubmit")}
                        </button>
                        <div className="account-center__reset-resend">
                          <RealCaptchaButton
                            api={api}
                            disabled={busy || resetSMSCooldownSeconds > 0}
                            title={t("account.passCaptcha")}
                            label={resetSMSCooldownSeconds > 0
                              ? t("account.resendSMSCountdown", { seconds: resetSMSCooldownSeconds })
                              : t("account.passwordResetResend")}
                            action="password-reset-resend"
                            hideTitle
                            onProof={sendPasswordResetSMS}
                            onError={reportCaptchaError}
                          />
                        </div>
                      </>
                    ) : (
                      <RealCaptchaButton
                        api={api}
                        disabled={busy || !phone.trim()}
                        title={t("account.passCaptcha")}
                        label={busy ? t("common.loading") : t("account.passwordResetSendCode")}
                        action="password-reset-send"
                        hideTitle
                        primary
                        onBeforeClick={validatePhone}
                        onProof={sendPasswordResetSMS}
                        onError={reportCaptchaError}
                      />
                    )
                  ) : (
                    <>
                      <AccountPasswordField
                        name="password"
                        label={t(mode === "register" ? "account.setPassword" : "account.password")}
                        autoComplete={mode === "login" ? "current-password" : "new-password"}
                        disabled={busy}
                        value={password}
                        required
                        error={fieldErrors.password}
                        errorId="account-password-error"
                        inputRef={passwordInputRef}
                        onChange={(value) => {
                          setPassword(value);
                          if (value) {
                            setFieldErrors((current) => current.password ? { ...current, password: undefined } : current);
                          }
                        }}
                      />
                      {mode === "register" ? (
                        <>
                          {registrationMode === "real" ? (
                            <RealCaptchaButton
                              api={api}
                              disabled={busy || smsCooldownSeconds > 0}
                              title={t("account.passCaptcha")}
                              label={smsCooldownSeconds > 0
                                ? t("account.resendSMSCountdown", { seconds: smsCooldownSeconds })
                              : t("account.sendSMS")}
                              action="captcha"
                              onBeforeClick={validatePhone}
                              onProof={sendSMS}
                              onError={reportCaptchaError}
                            />
                          ) : (
                            <div className="account-center__captcha">
                              <button className="account-center__button" type="button" disabled={busy || captchaPassed} onClick={() => {
                                if (!registrationAvailable) {
                                  setMode("login");
                                  setCaptchaPassed(false);
                                  return;
                                }
                                if (!validatePhone()) return;
                                setCaptchaPassed(true);
                              }} data-account-action="captcha">
                                <ShieldCheck size={14} aria-hidden="true" />
                                {captchaPassed ? t("account.captchaPassed") : t("account.passCaptcha")}
                              </button>
                              <button className="account-center__button" type="button" disabled={busy || !captchaPassed || smsCooldownSeconds > 0} onClick={() => void sendSMS()} data-account-action="sms">
                                {smsCooldownSeconds > 0
                                  ? t("account.resendSMSCountdown", { seconds: smsCooldownSeconds })
                                  : t("account.sendSMS")}
                              </button>
                            </div>
                          )}
                          <label className="account-center__field">
                            <span>
                              {t("account.smsCode")}
                              <span className="account-center__required" aria-hidden="true">*</span>
                            </span>
                            <input
                              ref={smsCodeInputRef}
                              name="sms_code"
                              inputMode="numeric"
                              autoComplete="one-time-code"
                              disabled={busy}
                              required
                              aria-invalid={Boolean(fieldErrors.smsCode)}
                              aria-describedby={fieldErrors.smsCode ? "account-sms-code-error" : undefined}
                              value={smsCode}
                              onChange={(event) => {
                                const value = event.currentTarget.value;
                                setSMSCode(value);
                                if (value.trim()) {
                                  setFieldErrors((current) => current.smsCode ? { ...current, smsCode: undefined } : current);
                                }
                              }}
                            />
                            {fieldErrors.smsCode
                              ? <span className="account-center__field-error" id="account-sms-code-error">{fieldErrors.smsCode}</span>
                              : null}
                          </label>
                          <button className="account-center__primary" type="button" disabled={busy} onClick={() => void register()} data-account-action="register">{busy ? t("common.loading") : t("account.register")}</button>
                        </>
                      ) : (
                        <>
                          {registrationMode === "real" ? (
                            <button
                              className="account-center__forgot-password"
                              type="button"
                              disabled={busy}
                              onClick={openPasswordReset}
                              data-account-action="password-reset-open"
                            >
                              {t("account.passwordResetOpen")}
                            </button>
                          ) : null}
                          {registrationMode === "real" ? (
                            <RealCaptchaButton
                              api={api}
                              disabled={busy}
                              title={t("account.passCaptcha")}
                              label={busy ? t("common.loading") : t("account.login")}
                              action="login"
                            hideTitle
                            primary
                            onBeforeClick={validateAuthentication}
                            onProof={login}
                              onError={reportCaptchaError}
                            />
                          ) : (
                            <button className="account-center__primary" type="button" disabled={busy} onClick={() => void login()} data-account-action="login">{busy ? t("common.loading") : t("account.login")}</button>
                          )}
                        </>
                      )}
                    </>
                  )}
                </div>
                <p className="account-center__auth-hint" data-account-auth-hint={registrationMode}>
                  <ShieldCheck size={15} aria-hidden="true" />
                  {t(mode === "password-reset"
                    ? resetCodeSent
                      ? "account.passwordResetConfirmHint"
                      : "account.passwordResetRequestHint"
                    : registrationMode === "fixture"
                      ? "account.authHint"
                      : mode === "login"
                        ? "account.realLoginHint"
                        : "account.realRegisterHint")}
                </p>
              </div>
            </div>
          </section>
        </div>
      )}
      {switchSection ? (
        <div
          className="account-center__switch-backdrop"
          role="presentation"
          onMouseDown={(event) => {
            if (event.target === event.currentTarget) cancelSwitch();
          }}
        >
          {switchSection}
        </div>
      ) : null}
    </section>
  );
}
