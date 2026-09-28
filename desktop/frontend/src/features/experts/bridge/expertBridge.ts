import { app, type AppBindings } from "../../../lib/bridge";

export type ExpertCatalogBindings = Pick<
  AppBindings,
  | "ListExperts"
  | "ListUserExperts"
  | "CurrentExpertForTab"
  | "ListUserExpertsForTab"
  | "ExpertLibraryStateForTab"
  | "CreateUserExpert"
  | "UpdateUserExpert"
  | "UpdateExpert"
  | "DuplicateExpert"
  | "SetExpertFavorite"
  | "ImportExpertConfig"
  | "DeleteUserExpert"
  | "DeleteExpert"
  | "ResetExperts"
>;

export const expertCatalogBridge: ExpertCatalogBindings = app;
