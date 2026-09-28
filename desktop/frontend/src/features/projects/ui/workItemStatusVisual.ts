import { Circle, CircleCheck, CirclePause, CirclePlay, type LucideIcon } from "lucide-react";
import type { CloudProjectWorkItemStatus } from "../model/projectTypes";

export const WORK_ITEM_STATUS_ICONS: Record<CloudProjectWorkItemStatus, LucideIcon> = {
  todo: Circle,
  in_progress: CirclePlay,
  paused: CirclePause,
  done: CircleCheck,
};
