import { useEffect, useRef } from "react";

export function useHorizontalDragScroll(primaryResetKey: unknown, secondaryResetKey?: unknown) {
	const scrollerRef = useRef<HTMLDivElement | null>(null);
	const dragRef = useRef<{ pointerId: number; startX: number; startScrollLeft: number; moved: boolean } | null>(null);
	const suppressClickRef = useRef(false);

	useEffect(() => {
		const scroller = scrollerRef.current;
		if (!scroller) return;
		let clickResetTimer: number | undefined;
		const handleWheel = (event: WheelEvent) => {
			const maxScrollLeft = scroller.scrollWidth - scroller.clientWidth;
			if (maxScrollLeft <= 0) return;
			const delta = Math.abs(event.deltaX) > Math.abs(event.deltaY) ? event.deltaX : event.deltaY;
			if (!delta) return;
			const nextScrollLeft = Math.max(0, Math.min(maxScrollLeft, scroller.scrollLeft + delta));
			if (nextScrollLeft === scroller.scrollLeft) return;
			event.preventDefault();
			scroller.scrollLeft = nextScrollLeft;
		};
		const finishDrag = (event: PointerEvent, suppressClick: boolean) => {
			const drag = dragRef.current;
			if (!drag || drag.pointerId !== event.pointerId) return;
			dragRef.current = null;
			scroller.classList.remove("is-dragging");
			if (typeof scroller.hasPointerCapture === "function" && scroller.hasPointerCapture(event.pointerId)) {
				scroller.releasePointerCapture(event.pointerId);
			}
			if (!suppressClick || !drag.moved) return;
			suppressClickRef.current = true;
			clickResetTimer = window.setTimeout(() => {
				suppressClickRef.current = false;
			}, 0);
		};
		const handlePointerDown = (event: PointerEvent) => {
			if (event.button !== 0 || (event.pointerType && event.pointerType !== "mouse")) return;
			if (scroller.scrollWidth <= scroller.clientWidth) return;
			dragRef.current = {
				pointerId: event.pointerId,
				startX: event.clientX,
				startScrollLeft: scroller.scrollLeft,
				moved: false,
			};
		};
		const handlePointerMove = (event: PointerEvent) => {
			const drag = dragRef.current;
			if (!drag || drag.pointerId !== event.pointerId) return;
			const deltaX = event.clientX - drag.startX;
			if (!drag.moved && Math.abs(deltaX) < 4) return;
			if (!drag.moved) {
				drag.moved = true;
				scroller.classList.add("is-dragging");
				if (typeof scroller.setPointerCapture === "function") scroller.setPointerCapture(event.pointerId);
			}
			event.preventDefault();
			const maxScrollLeft = scroller.scrollWidth - scroller.clientWidth;
			scroller.scrollLeft = Math.max(0, Math.min(maxScrollLeft, drag.startScrollLeft - deltaX));
		};
		const handleClickCapture = (event: MouseEvent) => {
			if (!suppressClickRef.current) return;
			suppressClickRef.current = false;
			event.preventDefault();
			event.stopPropagation();
		};
		const handlePointerUp = (event: PointerEvent) => finishDrag(event, true);
		const handlePointerCancel = (event: PointerEvent) => finishDrag(event, false);
		scroller.addEventListener("wheel", handleWheel, { passive: false });
		scroller.addEventListener("pointerdown", handlePointerDown);
		scroller.addEventListener("pointermove", handlePointerMove);
		scroller.addEventListener("pointerup", handlePointerUp);
		scroller.addEventListener("pointercancel", handlePointerCancel);
		scroller.addEventListener("click", handleClickCapture, true);
		return () => {
			window.clearTimeout(clickResetTimer);
			dragRef.current = null;
			suppressClickRef.current = false;
			scroller.classList.remove("is-dragging");
			scroller.removeEventListener("wheel", handleWheel);
			scroller.removeEventListener("pointerdown", handlePointerDown);
			scroller.removeEventListener("pointermove", handlePointerMove);
			scroller.removeEventListener("pointerup", handlePointerUp);
			scroller.removeEventListener("pointercancel", handlePointerCancel);
			scroller.removeEventListener("click", handleClickCapture, true);
		};
	}, [primaryResetKey, secondaryResetKey]);

	return scrollerRef;
}
