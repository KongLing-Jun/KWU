import {
  createContext,
  Fragment,
  forwardRef,
  isValidElement,
  type ReactNode,
  useCallback,
  useContext,
  useEffect,
  useImperativeHandle,
  useMemo,
  useRef,
} from "react";
import { useVirtualizer } from "@tanstack/react-virtual";
import { recordTranscriptTrace, TRANSCRIPT_TRACE_ENABLED } from "../lib/transcriptPerformanceTrace";

export const TRANSCRIPT_VIRTUALIZE_MIN_ROWS = 10;
const TRANSCRIPT_VIRTUAL_OVERSCAN = 3;
const TRANSCRIPT_VIRTUAL_ESTIMATED_ROW_HEIGHT = 180;
const TRANSCRIPT_VIRTUAL_INITIAL_VIEWPORT_HEIGHT = 800;

export type TranscriptVirtualRow = {
  key: string;
  node: ReactNode;
  anchorId?: string;
};

export type TranscriptVirtualRowsHandle = {
  scope: object;
  scrollToIndex: (index: number, align?: "auto" | "center" | "end" | "start") => void;
  visualOffsetForIndex: (index: number) => number | undefined;
  visualMetricsForIndex: (index: number) => { offset: number; listTop: number; rowStart: number } | undefined;
  isMeasured: (index: number) => boolean;
  layoutRevision: () => number;
};

export const TranscriptTraceViewContext = createContext("");

export function shouldVirtualizeTranscriptRows(rowCount: number): boolean {
  return rowCount >= TRANSCRIPT_VIRTUALIZE_MIN_ROWS;
}

export function transcriptVirtualRows(nodes: ReactNode[]): TranscriptVirtualRow[] {
  return nodes.map((node, index) => {
    const element = isValidElement<{ anchorId?: string }>(node) ? node : null;
    return {
      key: element?.key != null ? String(element.key) : `transcript-row-${index}`,
      node,
      anchorId: element?.props.anchorId,
    };
  });
}

export function transcriptVirtualRowMeasured(
  measuredRowKeys: ReadonlySet<string>,
  rows: readonly TranscriptVirtualRow[],
  index: number,
): boolean {
  const key = rows[index]?.key;
  return key !== undefined && measuredRowKeys.has(key);
}

type TranscriptRowsProps = {
  rows: TranscriptVirtualRow[];
  scrollElement: HTMLDivElement | null;
  testId?: string;
};

const VirtualizedTranscriptRows = forwardRef<TranscriptVirtualRowsHandle, TranscriptRowsProps>(function VirtualizedTranscriptRows({ rows, scrollElement, testId = "transcript-virtual-list" }, ref) {
  const traceView = useContext(TranscriptTraceViewContext);
  const handleScope = useRef({});
  const measuredRowKeys = useRef(new Set<string>());
  const layoutRevision = useRef(0);
  const previousRangeSignature = useRef("");
  const listRef = useRef<HTMLDivElement>(null);
  const virtualizer = useVirtualizer({
    count: rows.length,
    getScrollElement: () => scrollElement,
    useFlushSync: false,
    initialRect: {
      width: 0,
      height: scrollElement?.clientHeight || TRANSCRIPT_VIRTUAL_INITIAL_VIEWPORT_HEIGHT,
    },
    estimateSize: () => TRANSCRIPT_VIRTUAL_ESTIMATED_ROW_HEIGHT,
    getItemKey: (index) => rows[index]?.key ?? index,
    overscan: TRANSCRIPT_VIRTUAL_OVERSCAN,
  });

  useImperativeHandle(ref, () => ({
    scope: handleScope.current,
    scrollToIndex(index, align = "start") {
      virtualizer.scrollToIndex(index, { align });
    },
    visualOffsetForIndex(index) {
      const list = listRef.current;
      const offsetInfo = virtualizer.getOffsetForIndex(index, "start");
      if (!list || !scrollElement || !offsetInfo) return undefined;
      const [rowStart] = offsetInfo;
      const relativeTop = list.getBoundingClientRect().top
        - scrollElement.getBoundingClientRect().top
        + rowStart;
      return -relativeTop;
    },
    visualMetricsForIndex(index) {
      const list = listRef.current;
      const offsetInfo = virtualizer.getOffsetForIndex(index, "start");
      if (!list || !scrollElement || !offsetInfo) return undefined;
      const [rowStart] = offsetInfo;
      const listTop = list.getBoundingClientRect().top - scrollElement.getBoundingClientRect().top;
      return { offset: -(listTop + rowStart), listTop, rowStart };
    },
    isMeasured(index) {
      return transcriptVirtualRowMeasured(measuredRowKeys.current, rows, index);
    },
    layoutRevision() {
      return layoutRevision.current;
    },
  }), [rows, scrollElement, testId, virtualizer]);

  const virtualItems = virtualizer.getVirtualItems();
  const range = useMemo(() => ({
    first: virtualItems[0]?.index ?? -1,
    last: virtualItems[virtualItems.length - 1]?.index ?? -1,
    mounted: virtualItems.length,
  }), [virtualItems]);
  const rangeSignature = `${range.first}:${range.last}:${range.mounted}:${rows.length}`;
  if (previousRangeSignature.current && previousRangeSignature.current !== rangeSignature) {
    layoutRevision.current += 1;
  }
  previousRangeSignature.current = rangeSignature;

  useEffect(() => {
    recordTranscriptTrace("virtual.mount", {
      view: traceView,
      scope: testId,
      rowCount: rows.length,
      scrollTop: scrollElement?.scrollTop ?? -1,
      scrollHeight: scrollElement?.scrollHeight ?? -1,
      clientHeight: scrollElement?.clientHeight ?? -1,
    }, { persist: true });
    return () => {
      recordTranscriptTrace("virtual.unmount", {
        view: traceView,
        scope: testId,
        rowCount: rows.length,
        measured: measuredRowKeys.current.size,
      }, { persist: true });
    };
  }, [scrollElement, testId, traceView]);

  useEffect(() => {
    recordTranscriptTrace("virtual.range", {
      view: traceView,
      scope: testId,
      rowCount: rows.length,
      first: range.first,
      last: range.last,
      mounted: range.mounted,
      scrollTop: scrollElement?.scrollTop ?? -1,
      scrollHeight: scrollElement?.scrollHeight ?? -1,
      clientHeight: scrollElement?.clientHeight ?? -1,
    });
  }, [rangeSignature, scrollElement, testId, traceView]);

  const measureElement = useCallback((element: Element | null) => {
    if (element) layoutRevision.current += 1;
    if (element) {
      const index = Number((element as HTMLElement).dataset.index ?? -1);
      const key = rows[index]?.key;
      if (key !== undefined && !measuredRowKeys.current.has(key)) {
        measuredRowKeys.current.add(key);
        if (TRANSCRIPT_TRACE_ENABLED) {
          recordTranscriptTrace("virtual.measure", {
            view: traceView,
            scope: testId,
            index,
            measured: measuredRowKeys.current.size,
          });
        }
      }
    }
    virtualizer.measureElement(element);
  }, [rows, testId, traceView, virtualizer]);

  return (
    <div
      ref={listRef}
      className="transcript-virtual-list"
      data-testid={testId}
      data-row-count={rows.length}
      style={{ height: virtualizer.getTotalSize() }}
    >
      {virtualItems.map((virtualRow) => {
        const row = rows[virtualRow.index];
        return (
          <div
            key={row.key}
            ref={measureElement}
            data-index={virtualRow.index}
            className="transcript-virtual-row"
            style={{ transform: `translateY(${virtualRow.start}px)` }}
          >
            {row.node}
          </div>
        );
      })}
    </div>
  );
});

export const TranscriptRows = forwardRef<TranscriptVirtualRowsHandle, TranscriptRowsProps>(function TranscriptRows(props, ref) {
  if (!shouldVirtualizeTranscriptRows(props.rows.length)) {
    return <>{props.rows.map((row) => <Fragment key={row.key}>{row.node}</Fragment>)}</>;
  }
  return <VirtualizedTranscriptRows ref={ref} {...props} />;
});
