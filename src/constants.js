export const STUDENT_NAV_ITEMS = [
  ['Overview', '◫'], ['Attendance', '◷'], ['Marks', '▤'],
  ['CG calculator', '◉'], ['Room availability', '⌂'], ['Library', '▣'],
  ['Notices & circulars', '▧'], ['Fee details', '₹'],
];

export const FACULTY_NAV_ITEMS = [
  ['Faculty dashboard', '◫'], ['My classes', '▦'],
  ['Attendance register', '◷'], ['Room allocation', '⌂'], ['Marks entry', '▤'],
  ['Notices & circulars', '▧'], ['Room availability', '⌂'],
];

export const MANAGEMENT_NAV_ITEMS = [
  ['Fee details', '₹'], ['Library', '▣'], ['Room management', '⌂'], ['Notices & circulars', '▧'], ['Room availability', '⌂'],
];

export const TODAY = new Intl.DateTimeFormat('en-IN', {
  weekday: 'long',
  day: '2-digit',
  month: 'long',
  year: 'numeric',
}).format(new Date());
