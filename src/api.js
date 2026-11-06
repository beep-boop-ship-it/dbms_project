const API_BASE = (import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080/api/v1').replace(/\/$/, '');
const TIMEOUT_MS = 12000;

async function request(path, options = {}) {
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), TIMEOUT_MS);
  const token = localStorage.getItem('ims_token');
  const isForm = options.body instanceof FormData;
  const headers = { ...(isForm ? {} : { 'Content-Type': 'application/json' }), ...(token ? { Authorization: `Bearer ${token}` } : {}), ...options.headers };
  try {
    const response = await fetch(`${API_BASE}${path}`, { ...options, headers, signal: controller.signal });
    const body = response.status === 204 ? null : await response.json().catch(() => null);
    if (!response.ok) throw new Error(body?.error || `Request failed (${response.status})`);
    return body;
  } catch (error) {
    if (error.name === 'AbortError') throw new Error('The server took too long to respond. Try again.');
    if (error instanceof TypeError) throw new Error('Unable to reach the campus server. Check your connection and try again.');
    throw error;
  } finally { clearTimeout(timeout); }
}

export const api = {
  async login(credentials) { const data = await request('/auth/login', { method: 'POST', body: JSON.stringify(credentials) }); if (data?.token) localStorage.setItem('ims_token', data.token); return data; },
  logout() { localStorage.removeItem('ims_token'); localStorage.removeItem('ims_role'); },
  async get(resource) {
    const paths = { attendance: '/students/me/attendance', marks: '/students/me/marks', rooms: '/rooms', notices: '/notices', fees: '/students/me/fees', profile: '/auth/me', courses: '/faculty/me/courses', myIssues:'/students/me/library/issues', libraryBooks:'/management/library/books', libraryIssues:'/management/library/issues',currentAllocations:'/rooms/allocations/current',allocations:'/rooms/allocations',managementRooms:'/management/rooms' };
    const data = await request(paths[resource]);
    return data?.items ?? data;
  },
  updateAttendance(payload) { return request('/attendance', { method: 'POST', body: JSON.stringify(payload) }); },
  updateMarks(payload) { return request('/marks', { method: 'POST', body: JSON.stringify(payload) }); },
  createNotice(payload) { return request('/notices', { method: 'POST', body: JSON.stringify(payload) }); },
  updateFees(payload) { return request('/fees/current', { method: 'PUT', body: JSON.stringify(payload) }); },
  createBook(payload) { return request('/management/library/books', { method: 'POST', body: JSON.stringify(payload) }); },
  updateBook(payload) { return request(`/management/library/books/${encodeURIComponent(payload.id)}`, { method: 'PUT', body: JSON.stringify(payload) }); },
  issueBook(payload) { return request('/management/library/issues', { method: 'POST', body: JSON.stringify(payload) }); },
  returnBook(id) { return request(`/management/library/issues/${encodeURIComponent(id)}/return`, { method: 'POST', body: JSON.stringify({}) }); },
  async availableRooms({courseOfferingId,allocationDate,startTime,endTime}) { const q=new URLSearchParams({course_offering_id:courseOfferingId,allocation_date:allocationDate,start_time:startTime,end_time:endTime});const data=await request(`/rooms/available?${q}`);return data.items||[]; },
  claimRoom(payload) { return request('/rooms/allocations', {method:'POST',body:JSON.stringify(payload)}); },
  releaseRoom(id) { return request(`/rooms/allocations/${encodeURIComponent(id)}/release`,{method:'POST',body:JSON.stringify({})}); },
  createRoom(payload) { return request('/management/rooms',{method:'POST',body:JSON.stringify(payload)}); },
  updateRoom(payload) { return request(`/management/rooms/${encodeURIComponent(payload.id)}`,{method:'PUT',body:JSON.stringify(payload)}); },
  async courseStudents(id) { const data=await request(`/courses/${encodeURIComponent(id)}/students`);return data.items||[]; },
  async attendanceSessions(id) { const data=await request(`/attendance/sessions?course_offering_id=${encodeURIComponent(id)}`);return data.items||[]; },
  attendanceSession(id) { return request(`/attendance/sessions/${encodeURIComponent(id)}`); },
  createAttendanceSession(payload) { return request('/attendance/sessions',{method:'POST',body:JSON.stringify(payload)}); },
  updateAttendanceSession(payload) { return request(`/attendance/sessions/${encodeURIComponent(payload.id)}`,{method:'PUT',body:JSON.stringify(payload)}); },
};
