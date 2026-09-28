import { useRef, useState } from "react";
import type { CloudProject } from "../model/projectTypes";

type CloudAvailability = "anonymous" | "loading" | "available" | "unavailable" | "error";

interface CloudListState {
  ownerScope: string;
  availability: CloudAvailability;
  projectScope: string;
  items: CloudProject[];
  nextCursor: string | null;
  selected: CloudProject | null;
  stale: boolean;
  cachedAt: string | null;
  message: string;
}

interface JoinedCloudListState {
  ownerScope: string;
  availability: CloudAvailability;
  projectScope: string;
  items: CloudProject[];
  nextCursor: string | null;
  message: string;
}

export function initialProjectDirectoryState(ownerScope: string, authenticated: boolean): CloudListState {
  return {
    ownerScope,
    availability: authenticated ? "loading" : "anonymous",
    projectScope: "",
    items: [],
    nextCursor: null,
    selected: null,
    stale: false,
    cachedAt: null,
    message: "",
  };
}

export function initialJoinedProjectState(ownerScope: string, authenticated: boolean): JoinedCloudListState {
  return {
    ownerScope,
    availability: authenticated ? "loading" : "anonymous",
    projectScope: "",
    items: [],
    nextCursor: null,
    message: "",
  };
}

export function useProjectDirectoryController(accountScopeKey: string, authenticated: boolean) {
  const ownerScopeRef = useRef(accountScopeKey);
  const projectScopeRef = useRef("");
  const selectedProjectRef = useRef("");
  const featureRequestRef = useRef(0);
  const joinedRequestRef = useRef(0);
  const [state, setState] = useState(() => initialProjectDirectoryState(accountScopeKey, authenticated));
  const [joinedState, setJoinedState] = useState(() => initialJoinedProjectState(accountScopeKey, authenticated));
  const [query, setQuery] = useState("");
  const [appliedQuery, setAppliedQuery] = useState("");
  const [loadingMore, setLoadingMore] = useState(false);
  const [joinedLoadingMore, setJoinedLoadingMore] = useState(false);

  return {
    ownerScopeRef,
    projectScopeRef,
    selectedProjectRef,
    featureRequestRef,
    joinedRequestRef,
    state,
    setState,
    joinedState,
    setJoinedState,
    query,
    setQuery,
    appliedQuery,
    setAppliedQuery,
    loadingMore,
    setLoadingMore,
    joinedLoadingMore,
    setJoinedLoadingMore,
  };
}
