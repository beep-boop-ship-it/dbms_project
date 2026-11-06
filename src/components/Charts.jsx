import React, { useMemo } from 'react';

export function MarksBarChart({ rows }) {
  const subjects = rows.map(row => ({
    code: row.code,
    internal: row.max ? Math.round(row.internal / (row.max * .6) * 100) : 0,
    midterm: row.max ? Math.round(row.midterm / (row.max * .4) * 100) : 0,
  }));
  return <section className="card chart-card marks-chart-card"><div className="card-head"><div><h2>Assessment breakdown</h2><div className="subtle">Score as a percentage of each assessment maximum</div></div><div className="chart-legend"><span><i className="legend-internal"/>Internal</span><span><i className="legend-midterm"/>Midterm</span></div></div>
    <div className="bar-chart">{subjects.map(subject=><div className="bar-row" key={subject.code}><b>{subject.code}</b><div className="bar-pair"><div className="bar-track"><i className="bar-internal" style={{width:`${Math.min(subject.internal,100)}%`}}/><span>{subject.internal}%</span></div><div className="bar-track"><i className="bar-midterm" style={{width:`${Math.min(subject.midterm,100)}%`}}/><span>{subject.midterm}%</span></div></div></div>)}</div>
  </section>;
}

export function RoomStatusChart({ rooms }) {
  const counts = useMemo(() => [
    { label:'Available', color:'#4a8d68', count:rooms.filter(r=>r.status==='Available').length },
    { label:'Occupied', color:'#d89b55', count:rooms.filter(r=>r.status==='Occupied').length },
    { label:'Maintenance', color:'#c86f64', count:rooms.filter(r=>r.status==='Maintenance').length },
  ],[rooms]);
  const total = counts.reduce((sum,item)=>sum+item.count,0);
  let offset=0;
  const segments=counts.map(item=>{const length=total?item.count/total*100:0;const segment={...item,length,offset};offset+=length;return segment});
  return <section className="card room-summary"><div className="room-donut" style={{background:`conic-gradient(${segments.map(s=>`${s.color} ${s.offset}% ${s.offset+s.length}%`).join(', ')})`}}><div><b>{total}</b><small>rooms</small></div></div><div className="room-legend"><h2>Room status</h2><div className="subtle">Live campus availability</div>{counts.map(item=><div className="room-legend-row" key={item.label}><span><i style={{background:item.color}}/>{item.label}</span><b>{item.count}</b></div>)}</div></section>;
}
