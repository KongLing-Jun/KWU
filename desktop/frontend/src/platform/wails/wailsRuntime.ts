export interface WailsRuntime {
  EventsOn(name: string, cb: (...data: unknown[]) => void): () => void;
  BrowserOpenURL(url: string): void;
  WindowSetSystemDefaultTheme?(): void;
  WindowSetLightTheme?(): void;
  WindowSetDarkTheme?(): void;
  WindowSetBackgroundColour?(r: number, g: number, b: number, a: number): void;
  WindowGetSize?(): Promise<{ w: number; h: number }>;
  WindowGetPosition?(): Promise<{ x: number; y: number }>;
  WindowIsMaximised?(): Promise<boolean>;
  ClipboardSetText?(text: string): Promise<boolean>;
  OnFileDrop?(cb: (x: number, y: number, paths: string[]) => void, useDropTarget: boolean): void;
  OnFileDropOff?(): void;
}

export function getWailsRuntime(): WailsRuntime | undefined {
  return typeof window !== "undefined" ? window.runtime : undefined;
}
