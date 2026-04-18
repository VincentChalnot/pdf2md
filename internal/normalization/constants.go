package normalization

// CALIBRATION NEEDED — adjust these after visual inspection of debug output
const (
	DefaultLineSplitRatio = 3.0  // gap is structural if gap[i+1]/gap[i] >= this (elbow detection)
	MinWordsForElbow      = 4    // lines with fewer words skip elbow detection, rely on Pass B only
	MinStructuralLines    = 2    // min lines in a GapColumn to be confirmed structural
	LineNeighborhood      = 20.0 // pt: max Y distance to consider two gaps as vertically related
	AlignMargin           = 5.0  // pt: empty space threshold for left/right alignment detection
	JustifiedCoverage     = 0.85 // text must cover this fraction of container width
	MergeGapThreshold     = 6.0  // pt: max Y gap between lines to merge into same LogicalBlock
	CutGroupingTol        = 8.0  // pt: tolerance for matching gap XCenter positions
)
