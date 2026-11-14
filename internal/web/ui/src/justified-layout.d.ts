// The part of Flickr's justified-layout the timeline uses.
declare module "justified-layout" {
  type Box = { left: number; top: number; width: number; height: number };
  export default function justifiedLayout(
    ratios: number[],
    config: { containerWidth: number; containerPadding: number; boxSpacing: number; targetRowHeight: number },
  ): { containerHeight: number; boxes: Box[] };
}
