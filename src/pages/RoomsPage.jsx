import React, { useEffect, useMemo, useState } from 'react';
import PageTitle from '../components/PageTitle.jsx';
import { RoomStatusChart } from '../components/Charts.jsx';

export default function RoomsPage({ rooms }) {
  const [query,setQuery]=useState(''),[filter,setFilter]=useState('All');
  const visible=useMemo(()=>rooms.filter(r=>(filter==='All'||r.status===filter)&&`${r.room} ${r.block} ${r.floor} ${r.status}`.toLowerCase().includes(query.toLowerCase())),[rooms,filter,query]);
  return <><PageTitle eyebrow="Campus / facilities" title="Room availability" intro="Find an open classroom across Block A and Block B."/><div className="toolbar"><input className="search" value={query} onChange={e=>setQuery(e.target.value)} placeholder="Search rooms or blocks…"/><select className="select" value={filter} onChange={e=>setFilter(e.target.value)}>{['All','Available','Occupied','Maintenance'].map(x=><option key={x}>{x}</option>)}</select><span className="subtle">{rooms.filter(r=>r.status==='Available').length} rooms available</span></div><RoomStatusChart rooms={rooms}/><div className="room-grid">{visible.map(r=><div className={`room ${r.status.toLowerCase()}`} key={r.room}><div className="room-top"><b>{r.room}</b><span className="room-status">● {r.status}</span></div><small>Block {r.block} · {r.floor}</small><small>{r.use} · Capacity {r.capacity}</small></div>)}</div>{!visible.length&&<div className="empty-state card">No rooms match these filters.</div>}</>;
}
