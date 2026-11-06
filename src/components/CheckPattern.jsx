import React from 'react';

const marks = [
  [5, 14], [15, 77], [24, 35], [33, 88], [42, 18], [51, 61],
  [60, 39], [69, 82], [78, 17], [87, 55], [95, 91], [9, 49],
  [29, 6], [47, 94], [73, 67], [91, 28], [38, 52], [64, 8],
];

export default function CheckPattern() {
  return <div className="check-pattern" aria-hidden="true">
    {marks.map(([left, top], index) => <span
      key={index}
      style={{
        left: `${left}%`,
        top: `${top}%`,
        animationDelay: `${(index % 6) * -1.4}s`,
        animationDuration: `${13 + (index % 5) * 2}s`,
      }}
    >✓</span>)}
  </div>;
}
