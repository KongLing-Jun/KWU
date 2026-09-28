export type ComposerExpertResolution = {
  viewKey: string;
  tabId: string;
  refreshSequence: number;
  authoritative: boolean;
};

export function composerExpertShouldUseTargetIdentity({
  hasRenderedTarget,
  surfaceTransitionActive,
  optimisticExpertActivation,
  renderedViewKey,
  activeTabId,
  resolution,
}: {
  hasRenderedTarget: boolean;
  surfaceTransitionActive: boolean;
  optimisticExpertActivation?: boolean;
  renderedViewKey: string;
  activeTabId?: string;
  resolution: ComposerExpertResolution | null;
}): boolean {
  if (!hasRenderedTarget) return false;
  if (optimisticExpertActivation) return false;
  if (surfaceTransitionActive) return true;
  return !(
    resolution?.authoritative === true
    && resolution.viewKey === renderedViewKey
    && resolution.tabId === activeTabId
  );
}
