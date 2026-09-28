const pad = (part: number) => part.toString().padStart(2, "0");

/** Formats project timestamps in local time with second-level precision. */
export function formatProjectDateTime(value: string | number | Date, fallback = "—") {
  const date = value instanceof Date ? value : new Date(value);
  if (Number.isNaN(date.getTime())) return fallback;
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`;
}
