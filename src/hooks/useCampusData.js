import { useCallback, useEffect, useState } from 'react';
import { api } from '../api.js';

const emptyData = { attendance: [], marks: [], rooms: [], notices: [], fees: null, courses: [], myIssues: [], libraryBooks: [], libraryIssues: [] };
const emptyProfile = { name: '', id: '', role: 'student' };

export default function useCampusData() {
  const [token, setToken] = useState(localStorage.getItem('ims_token'));
  const [role, setRole] = useState(localStorage.getItem('ims_role') || 'student');
  const [profile, setProfile] = useState(emptyProfile);
  const [data, setData] = useState(emptyData);
  const [busy, setBusy] = useState(false);
  const [loading, setLoading] = useState(Boolean(localStorage.getItem('ims_token')));
  const [error, setError] = useState('');

  const refresh = useCallback(async () => {
    const profileResult = await api.get('profile');
    const authenticatedRole = profileResult.role;
    const resources = authenticatedRole === 'teacher'
      ? ['attendance', 'marks', 'rooms', 'notices', 'courses', 'currentAllocations','allocations']
      : authenticatedRole === 'management'
        ? ['rooms', 'notices', 'fees', 'courses', 'libraryBooks', 'libraryIssues','managementRooms','allocations']
        : ['attendance', 'marks', 'rooms', 'notices', 'fees', 'myIssues'];
    const values = await Promise.all(resources.map(async key => [key, await api.get(key)]));
    setProfile(profileResult);
    setRole(authenticatedRole);
    localStorage.setItem('ims_role', authenticatedRole);
    setData({ ...emptyData, ...Object.fromEntries(values) });
    setError('');
  }, []);

  useEffect(() => {
    if (!token) return;
    let mounted = true;
    setLoading(true);
    refresh().catch(cause => { if (mounted) { setError(cause.message); api.logout(); setToken(null); } })
      .finally(() => { if (mounted) setLoading(false); });
    return () => { mounted = false; };
  }, [token, refresh]);

  const login = async credentials => {
    setBusy(true); setError('');
    try {
      const result = await api.login(credentials);
      if (!result?.token || !result?.user) throw new Error('Sign in did not return a valid session.');
      setRole(result.user.role); setProfile(result.user); setToken(result.token);
    } catch (cause) { setError(cause.message || 'Unable to sign in.'); }
    finally { setBusy(false); }
  };

  const logout = () => { api.logout(); setToken(null); setProfile(emptyProfile); setData(emptyData); };
  const mutate = async (operation, success) => { const result=await operation(); await refresh(); if (success) success(); return result; };
  const updateAttendance = change => mutate(() => api.updateAttendance(change));
  const updateMarks = change => mutate(() => api.updateMarks(change));
  const uploadNotice = ({ title, type, file }) => mutate(async () => {
    if (file) { const form = new FormData(); form.append('title', title); form.append('type', type); form.append('file', file); await api.uploadCircular(form); }
    else await api.createNotice({ title, type });
  });
  const updateFees = update => mutate(() => api.updateFees({ ...data.fees, amount: Number(update.amount), due: update.due }));
  const createBook = book => mutate(() => api.createBook(book));
  const updateBook = book => mutate(() => api.updateBook(book));
  const issueBook = issue => mutate(() => api.issueBook(issue));
  const returnBook = id => mutate(() => api.returnBook(id));
  const claimRoom = allocation => mutate(() => api.claimRoom(allocation));
  const releaseRoom = id => mutate(() => api.releaseRoom(id));
  const createRoom = room => mutate(() => api.createRoom(room));
  const updateRoom = room => mutate(() => api.updateRoom(room));
  const availableRooms = filters => api.availableRooms(filters);
  const getCourseStudents = id => api.courseStudents(id);
  const getAttendanceSessions = id => api.attendanceSessions(id);
  const getAttendanceSession = id => api.attendanceSession(id);
  const saveAttendanceSession = (payload,update) => mutate(() => update?api.updateAttendanceSession(payload):api.createAttendanceSession(payload));

  return { token, role, setRole, profile, data, busy, loading, error, setError, login, logout, updateAttendance, updateMarks, uploadNotice, updateFees, createBook, updateBook, issueBook, returnBook, claimRoom, releaseRoom, createRoom, updateRoom, availableRooms, getCourseStudents, getAttendanceSessions, getAttendanceSession, saveAttendanceSession };
}
