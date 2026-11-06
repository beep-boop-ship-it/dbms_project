import React, { useEffect, useState } from 'react';
import useCampusData from './hooks/useCampusData.js';
import Header from './components/Header.jsx';
import Sidebar from './components/Sidebar.jsx';
import Toast from './components/Toast.jsx';
import LoginPage from './pages/LoginPage.jsx';
import OverviewPage from './pages/OverviewPage.jsx';
import AttendancePage from './pages/AttendancePage.jsx';
import MarksPage from './pages/MarksPage.jsx';
import CgPage from './pages/CgPage.jsx';
import RoomsPage from './pages/RoomsPage.jsx';
import NoticesPage from './pages/NoticesPage.jsx';
import FeesPage from './pages/FeesPage.jsx';
import FacultyDashboardPage from './pages/FacultyDashboardPage.jsx';
import FacultyClassesPage from './pages/FacultyClassesPage.jsx';
import LibraryPage from './pages/LibraryPage.jsx';
import TeacherAttendancePage from './pages/TeacherAttendancePage.jsx';
import RoomAllocationPage from './pages/RoomAllocationPage.jsx';
import CheckPattern from './components/CheckPattern.jsx';

export default function App() {
  const campus = useCampusData();
  const [page, setPage] = useState(localStorage.getItem('ims_role') === 'teacher' ? 'Faculty dashboard' : 'Overview');
  const [toast, setToast] = useState('');
  const [mobileOpen, setMobileOpen] = useState(false);
  const navigate = name => { setPage(name); setMobileOpen(false); };
  const notify = message => setToast(message);
  useEffect(() => { if (campus.token) setPage(campus.role === 'teacher' ? 'Faculty dashboard' : campus.role === 'management' ? 'Fee details' : 'Overview'); }, [campus.token, campus.role]);

  if (campus.loading) return <div className="loading-screen" role="status">Restoring your campus session…</div>;
  if (!campus.token) return <><LoginPage onLogin={campus.login} busy={campus.busy} error={campus.error}/><Toast message={toast} onDone={() => setToast('')}/></>;

  let content;
  switch (page) {
    case 'Faculty dashboard': content = <FacultyDashboardPage data={campus.data} profile={campus.profile} navigate={navigate}/>; break;
    case 'My classes': content = <FacultyClassesPage courses={campus.data.courses} attendance={campus.data.attendance} marks={campus.data.marks} navigate={navigate}/>; break;
    case 'Attendance register': content = <TeacherAttendancePage courses={campus.data.courses} loadStudents={campus.getCourseStudents} loadSessions={campus.getAttendanceSessions} loadSession={campus.getAttendanceSession} saveSession={campus.saveAttendanceSession} onSuccess={notify}/>; break;
    case 'Marks entry': content = <MarksPage rows={campus.data.marks} role="teacher" onSave={change => campus.updateMarks(change).then(() => notify('Marks updated.')).catch(e => notify(e.message))}/>; break;
    case 'Attendance': content = <AttendancePage rows={campus.data.attendance} role={campus.role} onSave={change => campus.updateAttendance(change).then(() => notify('Attendance updated.')).catch(e => notify(e.message))}/>; break;
    case 'Marks': content = <MarksPage rows={campus.data.marks} role={campus.role} onSave={change => campus.updateMarks(change).then(() => notify('Marks updated.')).catch(e => notify(e.message))}/>; break;
    case 'CG calculator': content = <CgPage/>; break;
    case 'Room availability': content = <RoomsPage rooms={campus.data.rooms}/>; break;
    case 'Notices & circulars': content = <NoticesPage notices={campus.data.notices} role={campus.role} onUpload={payload => campus.uploadNotice(payload).then(() => notify('Circular published.')).catch(e => notify(e.message))}/>; break;
    case 'Fee details': content = campus.data.fees ? <FeesPage fees={campus.data.fees} role={campus.role} onUpdate={update => campus.updateFees(update).then(() => notify('Latest fee details published.')).catch(e => notify(e.message))}/> : <div className="empty-state">Fee records are not available for this account.</div>; break;
    case 'Library': content = <LibraryPage role={campus.role} books={campus.data.libraryBooks} issues={campus.role==='management'?campus.data.libraryIssues:campus.data.myIssues} onCreateBook={book=>campus.createBook(book).then(()=>notify('Book added to inventory.')).catch(e=>{notify(e.message);throw e})} onUpdateBook={book=>campus.updateBook(book).then(()=>notify('Book record updated.')).catch(e=>{notify(e.message);throw e})} onIssue={issue=>campus.issueBook(issue).then(()=>notify('Book issue recorded.')).catch(e=>{notify(e.message);throw e})} onReturn={id=>campus.returnBook(id).then(()=>notify('Book return recorded.')).catch(e=>notify(e.message))}/>; break;
    case 'Room allocation': content = <RoomAllocationPage role={campus.role} courses={campus.data.courses} rooms={campus.data.rooms} current={campus.data.currentAllocations||[]} allocations={campus.data.allocations||[]} loadAvailable={campus.availableRooms} onClaim={campus.claimRoom} onRelease={campus.releaseRoom} onSuccess={notify}/>; break;
    case 'Room management': content = <RoomAllocationPage role={campus.role} rooms={campus.data.managementRooms||campus.data.rooms} allocations={campus.data.allocations||[]} loadAvailable={campus.availableRooms} onSaveRoom={room=>room.id?campus.updateRoom(room):campus.createRoom(room)} onSuccess={notify}/>; break;
    default: content = <OverviewPage data={campus.data} profile={campus.profile} navigate={navigate}/>;
  }

  return <><div className="shell"><CheckPattern/><Sidebar page={page} profile={campus.profile} role={campus.role} onNavigate={navigate} open={mobileOpen} onClose={() => setMobileOpen(false)}/><main className="main"><Header page={page} role={campus.role} onMenu={() => setMobileOpen(open => !open)} onLogout={campus.logout}/><div className="content">{campus.error && <div className="form-error" role="alert">{campus.error} <button className="text-btn" onClick={() => window.location.reload()}>Reload</button></div>}{content}</div></main></div><Toast message={toast} onDone={() => setToast('')}/></>;
}
