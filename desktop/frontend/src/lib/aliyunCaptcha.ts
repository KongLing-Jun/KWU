export const ALIYUN_CAPTCHA_PROVIDER = "aliyun_captcha_v2";
export const ALIYUN_CAPTCHA_SCRIPT_URL = "https://o.alicdn.com/captcha-frontend/aliyunCaptcha/AliyunCaptcha.js";

const MAX_CAPTCHA_TICKET_BYTES = 8192;
const CONFIG_FIELDS = new Set(["provider", "region", "prefix", "scene_id"]);

export interface AliyunCaptchaConfig {
  provider: typeof ALIYUN_CAPTCHA_PROVIDER;
  region: "cn";
  prefix: string;
  scene_id: string;
}

export interface AliyunCaptchaProof {
  ticket: string;
  provider: typeof ALIYUN_CAPTCHA_PROVIDER;
}

interface AliyunCaptchaInstance {
  destroy?: () => void;
}

export interface AliyunCaptchaSDKOptions {
  SceneId: string;
  mode: "popup";
  element: string;
  button: string;
  success?: (captchaVerifyParam: string) => void;
  fail?: (error: unknown) => void;
  getInstance?: (instance: AliyunCaptchaInstance) => void;
}

type AliyunWindow = Window & typeof globalThis & {
  AliyunCaptchaConfig?: { region: string; prefix: string };
  initAliyunCaptcha?: (options: AliyunCaptchaSDKOptions) => void;
};

function strictPublicValue(value: unknown, field: string): string {
  if (typeof value !== "string" || value.length === 0 || value.trim() !== value) {
    throw new Error(`invalid CAPTCHA ${field}`);
  }
  return value;
}

export function validateAliyunCaptchaConfig(value: unknown): AliyunCaptchaConfig {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    throw new Error("invalid CAPTCHA config");
  }
  const record = value as Record<string, unknown>;
  if (Object.keys(record).some((key) => !CONFIG_FIELDS.has(key))) {
    throw new Error("invalid CAPTCHA config fields");
  }
  if (record.provider !== ALIYUN_CAPTCHA_PROVIDER || record.region !== "cn") {
    throw new Error("unsupported CAPTCHA config");
  }
  const prefix = strictPublicValue(record.prefix, "prefix");
  const sceneID = strictPublicValue(record.scene_id, "scene_id");
  if (prefix === "reasonix-dev" || sceneID === "reasonix-register-dev") {
    throw new Error("development CAPTCHA config is not allowed");
  }
  return { provider: ALIYUN_CAPTCHA_PROVIDER, region: "cn", prefix, scene_id: sceneID };
}

let scriptLoad: Promise<void> | null = null;

export function preloadAliyunCaptcha(
  targetWindow: Window & typeof globalThis = window,
  targetDocument: Document = document,
): Promise<void> {
  const sdkWindow = targetWindow as AliyunWindow;
  if (typeof sdkWindow.initAliyunCaptcha === "function") return Promise.resolve();
  if (scriptLoad) return scriptLoad;
  targetDocument.querySelector<HTMLScriptElement>(`script[src="${ALIYUN_CAPTCHA_SCRIPT_URL}"]`)?.remove();
  scriptLoad = new Promise<void>((resolve, reject) => {
    const script = targetDocument.createElement("script");
    const discard = () => {
      script.removeEventListener("load", loaded);
      script.removeEventListener("error", failed);
      if (script.src === ALIYUN_CAPTCHA_SCRIPT_URL) script.remove();
      scriptLoad = null;
    };
    const loaded = () => {
      if (typeof sdkWindow.initAliyunCaptcha !== "function") {
        discard();
        reject(new Error("Aliyun CAPTCHA SDK did not initialize"));
        return;
      }
      script.removeEventListener("error", failed);
      resolve();
    };
    const failed = () => {
      discard();
      reject(new Error("Aliyun CAPTCHA SDK failed to load"));
    };
    script.addEventListener("load", loaded, { once: true });
    script.addEventListener("error", failed, { once: true });
    script.src = ALIYUN_CAPTCHA_SCRIPT_URL;
    script.async = true;
    targetDocument.head.append(script);
  });
  return scriptLoad;
}

interface AliyunCaptchaAdapterOptions {
  config: AliyunCaptchaConfig;
  window?: Window & typeof globalThis;
  document?: Document;
  element: HTMLElement;
  button: HTMLButtonElement;
  onProof: (proof: AliyunCaptchaProof) => Promise<void> | void;
  onError: (error: unknown) => void;
}

let captchaElementSequence = 0;

export class AliyunCaptchaAdapter {
  private readonly config: AliyunCaptchaConfig;
  private readonly sdkWindow: AliyunWindow;
  private readonly sdkDocument: Document;
  private readonly element: HTMLElement;
  private readonly button: HTMLButtonElement;
  private readonly onProof: AliyunCaptchaAdapterOptions["onProof"];
  private readonly onError: AliyunCaptchaAdapterOptions["onError"];
  private instance: AliyunCaptchaInstance | null = null;
  private intent = false;
  private consuming = false;
  private disposed = false;
  private rebuilding = false;

  constructor(options: AliyunCaptchaAdapterOptions) {
    this.config = options.config;
    this.sdkWindow = (options.window ?? window) as AliyunWindow;
    this.sdkDocument = options.document ?? document;
    this.element = options.element;
    this.button = options.button;
    this.onProof = options.onProof;
    this.onError = options.onError;
    captchaElementSequence += 1;
    this.element.id ||= `aliyun-captcha-${captchaElementSequence}`;
    this.button.id ||= `aliyun-captcha-button-${captchaElementSequence}`;
  }

  async init(): Promise<void> {
    if (this.disposed) return;
    await preloadAliyunCaptcha(this.sdkWindow, this.sdkDocument);
    if (this.disposed) return;
    this.button.addEventListener("click", this.captureIntent, true);
    this.initializeSDK();
  }

  destroy(): void {
    this.disposed = true;
    this.intent = false;
    this.consuming = false;
    this.rebuilding = false;
    this.button.removeEventListener("click", this.captureIntent, true);
    this.destroyInstance();
  }

  private readonly captureIntent = () => {
    if (!this.disposed && !this.consuming && !this.rebuilding && !this.button.disabled) this.intent = true;
  };

  private initializeSDK(): void {
    const initialize = this.sdkWindow.initAliyunCaptcha;
    if (!initialize) throw new Error("Aliyun CAPTCHA SDK is unavailable");
    this.sdkWindow.AliyunCaptchaConfig = { region: this.config.region, prefix: this.config.prefix };
    initialize({
      SceneId: this.config.scene_id,
      mode: "popup",
      element: `#${this.element.id}`,
      button: `#${this.button.id}`,
      success: (ticket) => this.handleSuccess(ticket),
      fail: (error) => this.handleFailure(error),
      getInstance: (instance) => { this.instance = instance; },
    });
  }

  private handleSuccess(ticket: string): void {
    if (this.disposed || this.consuming || !this.intent) return;
    this.intent = false;
    this.consuming = true;
    if (typeof ticket !== "string" || ticket.trim() === "" || new TextEncoder().encode(ticket).length > MAX_CAPTCHA_TICKET_BYTES) {
      this.onError(new Error("invalid CAPTCHA proof"));
      void this.rebuild();
      return;
    }
    void Promise.resolve(this.onProof({ ticket, provider: ALIYUN_CAPTCHA_PROVIDER }))
      .catch(this.onError)
      .finally(() => this.rebuild());
  }

  private handleFailure(error: unknown): void {
    if (this.disposed || this.consuming || !this.intent) return;
    this.intent = false;
    this.consuming = true;
    this.onError(error);
    void this.rebuild();
  }

  private async rebuild(): Promise<void> {
    if (this.disposed || this.rebuilding) return;
    this.rebuilding = true;
    this.destroyInstance();
    try {
      await preloadAliyunCaptcha(this.sdkWindow, this.sdkDocument);
      if (!this.disposed) this.initializeSDK();
    } catch (error) {
      if (!this.disposed) this.onError(error);
    } finally {
      this.rebuilding = false;
      if (!this.disposed) this.consuming = false;
    }
  }

  private destroyInstance(): void {
    const instance = this.instance;
    this.instance = null;
    instance?.destroy?.();
  }
}
