import bB from "@assets/bB.svg";
import bK from "@assets/bK.svg";
import bN from "@assets/bN.svg";
import bP from "@assets/bP.svg";
import bQ from "@assets/bQ.svg";
import bR from "@assets/bR.svg";
import wB from "@assets/wB.svg";
import wK from "@assets/wK.svg";
import wN from "@assets/wN.svg";
import wP from "@assets/wP.svg";
import wQ from "@assets/wQ.svg";
import wR from "@assets/wR.svg";

export const squares = [
  "a8",
  "b8",
  "c8",
  "d8",
  "e8",
  "f8",
  "g8",
  "h8",
  "a7",
  "b7",
  "c7",
  "d7",
  "e7",
  "f7",
  "g7",
  "h7",
  "a6",
  "b6",
  "c6",
  "d6",
  "e6",
  "f6",
  "g6",
  "h6",
  "a5",
  "b5",
  "c5",
  "d5",
  "e5",
  "f5",
  "g5",
  "h5",
  "a4",
  "b4",
  "c4",
  "d4",
  "e4",
  "f4",
  "g4",
  "h4",
  "a3",
  "b3",
  "c3",
  "d3",
  "e3",
  "f3",
  "g3",
  "h3",
  "a2",
  "b2",
  "c2",
  "d2",
  "e2",
  "f2",
  "g2",
  "h2",
  "a1",
  "b1",
  "c1",
  "d1",
  "e1",
  "f1",
  "g1",
  "h1",
];

const pieceMap: Record<string, string> = {
  wp: wP,
  wn: wN,
  wb: wB,
  wr: wR,
  wq: wQ,
  wk: wK,
  bp: bP,
  bn: bN,
  bb: bB,
  br: bR,
  bq: bQ,
  bk: bK,
};

export function pieceToAsset(color: "w" | "b", type: string): string {
  return pieceMap[`${color}${type}`];
}
