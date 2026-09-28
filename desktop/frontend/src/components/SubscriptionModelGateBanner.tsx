import { CircleAlert } from "lucide-react";
import { useMemo } from "react";

import { useT } from "../lib/i18n";
import {
  filterModelInfosForSubscriptionAccess,
  type SubscriptionAccess,
  type SubscriptionModelInvocationAccess,
} from "../lib/subscriptionModelAccess";
import type { ModelInfo, SharedModelCatalogState } from "../lib/types";

const unavailableSubscriptionAccess: SubscriptionAccess = { state: "unavailable" };
const unavailableModelCatalog: SharedModelCatalogState = { state: "unavailable" };

export function SubscriptionModelGateBanner({
  access,
  modelCatalog = unavailableModelCatalog,
  subscriptionAccess = unavailableSubscriptionAccess,
  onSwitchModel,
  onOpenModelPicker,
}: {
  access: SubscriptionModelInvocationAccess;
  modelCatalog?: SharedModelCatalogState;
  subscriptionAccess?: SubscriptionAccess;
  onSwitchModel: (ref: string, billingSource?: ModelInfo["billingSource"]) => void | Promise<void>;
  onOpenModelPicker?: () => void;
}) {
  const t = useT();
  const models = useMemo(() => access.state === "blocked" && modelCatalog.state === "ready"
    ? filterModelInfosForSubscriptionAccess(
        modelCatalog.routes,
        modelCatalog.marketplace,
        subscriptionAccess,
      ).filter((model) => model.ref && !model.disabled && !model.current)
    : [], [access.state, modelCatalog, subscriptionAccess]);
  const loading = modelCatalog.state === "loading";
  if (access.state !== "blocked") return null;
  return (
    <div
      className="send-availability banner banner--warning"
      role="status"
      data-subscription-model-gate="blocked"
    >
      <div className="send-availability__copy">
        <span className="send-availability__icon" aria-hidden="true">
          <CircleAlert size={17} />
        </span>
        <span className="banner__msg">
          {t("subscriptionModelGate.blockedMcpUnaffected")}
        </span>
      </div>
      <div className="send-availability__actions">
        {models.slice(0, 3).map((model) => (
          <button
            className="btn btn--small btn--primary"
            type="button"
            key={`${model.billingSource ?? ""}:${model.ref}`}
            onClick={() => void onSwitchModel(model.ref, model.billingSource)}
          >
            {t("subscriptionModelGate.switchTo", { model: model.displayName || model.model || model.ref })}
          </button>
        ))}
        {!loading && models.length === 0 && <span>{t("subscriptionModelGate.noUsableModels")}</span>}
        <button className="btn btn--small" type="button" onClick={() => onOpenModelPicker?.()}>
          {t("subscriptionModelGate.manageModels")}
        </button>
      </div>
    </div>
  );
}
