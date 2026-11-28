// @nithron/tokens 0.4.0 (the line tokens of dist/index.js, Nithron Community License v1.0). Copied whole from the nithron-site repository; replace it whole with a new release, never edit it here.
/* eslint-disable quotes, comma-dangle -- a copy, replaced whole with a new release: it keeps the package's own quotes. */
// @nithron/tokens: generated from src/tokens.json. Do not edit.
export const line = {
  "$description": "The line direction (DESIGN-V3.md, approved by Thiago on 2026-11-27 on the prototype in design/v3/samples/line). Surfaces are black or white and follow the visitor's system; pictures are fine-line isometric figures and drawings made of monospace characters; colour is small and functional, inside pictures and marks only. Emitted to dist/line.css, the file other Nithron repositories import. A page loads one of tokens.css, v3.css or line.css, never two. The real pages in apps/ still load tokens.css or v3.css until they are rebuilt. `light` and `dark` hold the greys of each ground; `colour` the hues in their four uses on each ground; `main` and `status` name which hue plays which part.",
  "light": {
    "page": "#f6f7f8",
    "ink": "#131517",
    "ink-2": "#4c5056",
    "dim": "#62666c",
    "hairline": "#dfe1e4",
    "rule": "#c2c5ca",
    "edge": "#6f7379",
    "figure-line": "#1c1e21",
    "figure-line-2": "#6f7379",
    "reference": "#131517"
  },
  "dark": {
    "page": "#0c0d0f",
    "ink": "#eceef0",
    "ink-2": "#a9adb3",
    "dim": "#84888e",
    "hairline": "#1f2124",
    "rule": "#34373c",
    "edge": "#7a7e85",
    "figure-line": "#b9bdc3",
    "figure-line-2": "#7a7e85",
    "reference": "#eceef0"
  },
  "colour": {
    "$description": "A hue in its four uses on the soft dark and the soft light ground. line: a line or a mark (3:1 or more). text: small text and code (4.5:1 or more). tint: a quiet fill; text on it is ink only. bar: the bar of a chart series or a meter block. neutral is ink: it is what the interface itself uses, since the interface has no hue (Thiago, 2026-11-27: \"Grey interface\"). The five hues blue, cyan, gold, orange and violet are an editor's syntax palette, one family in OKLCH (design/v3/samples/minimal/lib/palette.mjs), and appear only in code, logs, diagrams and chart series. green says passed and red says failed, nowhere else. No pink, bronze, lime or teal exists in this palette.",
    "neutral": {
      "dark": {
        "line": "#eceef0",
        "text": "#eceef0",
        "tint": "#2b2d2f",
        "bar": "#eceef0"
      },
      "light": {
        "line": "#131517",
        "text": "#131517",
        "tint": "#dfe0e2",
        "bar": "#131517"
      }
    },
    "blue": {
      "dark": {
        "line": "#74acff",
        "text": "#74acff",
        "tint": "#233044",
        "bar": "#74acff"
      },
      "light": {
        "line": "#1f5dc2",
        "text": "#1f5dc2",
        "tint": "#d4e4fa",
        "bar": "#1f5dc2"
      }
    },
    "cyan": {
      "dark": {
        "line": "#6fd2fc",
        "text": "#6fd2fc",
        "tint": "#223843",
        "bar": "#6fd2fc"
      },
      "light": {
        "line": "#0371a0",
        "text": "#0371a0",
        "tint": "#d3edf9",
        "bar": "#0371a0"
      }
    },
    "gold": {
      "dark": {
        "line": "#f0c551",
        "text": "#f0c551",
        "tint": "#3e351e",
        "bar": "#f0c551"
      },
      "light": {
        "line": "#916304",
        "text": "#916304",
        "tint": "#f4eacd",
        "bar": "#916304"
      }
    },
    "orange": {
      "dark": {
        "line": "#ff963d",
        "text": "#ff963d",
        "tint": "#412b19",
        "bar": "#ff963d"
      },
      "light": {
        "line": "#c04502",
        "text": "#c04502",
        "tint": "#f8dec7",
        "bar": "#c04502"
      }
    },
    "violet": {
      "dark": {
        "line": "#9c9afe",
        "text": "#9c9afe",
        "tint": "#2c2c44",
        "bar": "#9c9afe"
      },
      "light": {
        "line": "#5a43d4",
        "text": "#5a43d4",
        "tint": "#dfdffa",
        "bar": "#5a43d4"
      }
    },
    "green": {
      "dark": {
        "line": "#54dd7d",
        "text": "#54dd7d",
        "tint": "#1c3b27",
        "bar": "#54dd7d"
      },
      "light": {
        "line": "#017b37",
        "text": "#017b37",
        "tint": "#ccf0d8",
        "bar": "#017b37"
      }
    },
    "red": {
      "dark": {
        "line": "#ff413d",
        "text": "#ff413d",
        "tint": "#411819",
        "bar": "#ff413d"
      },
      "light": {
        "line": "#d40c1a",
        "text": "#d40c1a",
        "tint": "#f8c8c7",
        "bar": "#d40c1a"
      }
    }
  },
  "main": {
    "$description": "The interface has no hue: both sites and every product read ink here. --main and its three other uses stay as variables so nothing that maps onto them breaks; data-site=\"lab\" changes nothing any more.",
    "studio": "neutral",
    "lab": "neutral"
  },
  "status": {
    "$description": "The three states that carry a hue, by their name in colour. pass: passed, in order, done (a true green, used for nothing else). earlier: a warning, something pending or provisional (gold; in a product, name it \"warning\" in your mapping file). failed: what went wrong (red), always with a second cue such as a cross and a word.",
    "pass": "green",
    "earlier": "gold",
    "failed": "red"
  },
  "syntax": {
    "$description": "An editor's syntax palette for code, logs and diagrams: --syntax-keyword and so on, as text colours. comment is the quietest grey that is still text.",
    "keyword": "blue",
    "function": "cyan",
    "string": "gold",
    "number": "orange",
    "type": "violet",
    "comment": "dim"
  },
  "series": {
    "1": "blue",
    "2": "cyan",
    "3": "gold",
    "4": "orange",
    "5": "violet",
    "$description": "Chart series beyond the subject and its reference, in this order: --series-1 (a line) and --series-1-bar, up to five. Tell more series apart by the style of the line, not by more hues."
  },
  "font": {
    "$description": "sans and mono are the two text faces. pixel is the display face of the pixel layer (a product's name, a screen's one large number, the words of an empty or loading screen): Nithron Pixel, fonts/nithron-pixel.woff2 in this package, with the mono stack behind it. Set it only at whole multiples of 8 px.",
    "sans": "\"Mona Sans Variable\", ui-sans-serif, system-ui, -apple-system, \"Segoe UI\", sans-serif",
    "mono": "\"Monaspace Neon\", ui-monospace, SFMono-Regular, Menlo, Consolas, monospace",
    "pixel": "\"Nithron Pixel\", \"Monaspace Neon\", ui-monospace, SFMono-Regular, Menlo, Consolas, monospace"
  },
  "type": {
    "$description": "The type scale of the prototype. h1 to body, small and fine are set in the sans face; mono and label in the mono face. Sizes in rem follow the root size (layout.root-size on the sites, 16 px in an app).",
    "h1": {
      "size": "clamp(2.875rem, 6.2vw, 6.25rem)",
      "leading": "0.96",
      "weight": "600",
      "tracking": "-0.04em"
    },
    "h2": {
      "size": "clamp(1.9375rem, 3.1vw, 3.75rem)",
      "leading": "1.04",
      "weight": "580",
      "tracking": "-0.03em"
    },
    "h2-sub": {
      "size": "clamp(1.5rem, 2.2vw, 2.625rem)",
      "leading": "1.04",
      "weight": "580",
      "tracking": "-0.03em"
    },
    "h3": {
      "size": "clamp(1.25rem, 1.7vw, 1.875rem)",
      "leading": "1.2",
      "weight": "560",
      "tracking": "-0.015em"
    },
    "title": {
      "size": "1.25rem",
      "leading": "1.25",
      "weight": "560",
      "tracking": "-0.012em"
    },
    "number": {
      "size": "clamp(1.75rem, 2.6vw, 3rem)",
      "leading": "1",
      "weight": "580",
      "tracking": "-0.03em"
    },
    "lead": {
      "size": "clamp(1.1875rem, 1.45vw, 1.5rem)",
      "leading": "1.42",
      "weight": "400",
      "tracking": "0em"
    },
    "body": {
      "size": "1.0625rem",
      "leading": "1.5",
      "weight": "400",
      "tracking": "0em"
    },
    "small": {
      "size": "0.9375rem",
      "leading": "1.45",
      "weight": "400",
      "tracking": "0em"
    },
    "fine": {
      "size": "0.875rem",
      "leading": "1.5",
      "weight": "400",
      "tracking": "0em"
    },
    "mono": {
      "size": "0.875rem",
      "leading": "1.5",
      "weight": "400",
      "tracking": "0em"
    },
    "label": {
      "size": "0.8125rem",
      "leading": "1.5",
      "weight": "400",
      "tracking": "0em"
    }
  },
  "weight": {
    "$description": "The weight of a button, a name inside running text and the current item of a menu.",
    "strong": "560"
  },
  "layout": {
    "$description": "The ruled sheet. columns: equal columns; a cell takes 12, 6, 4 or 3 of them. wrap: the widest the sheet gets. page-pad: between the window and the sheet. cell-pad: inside every cell. section-gap: between sections. root-size: the size of 1rem on the sites (16 px up to a 1440 px window, 20 px at 2720 px); an app keeps 16 px. The three measures keep text lines short. control-height: a button, a field, a switch; control-height-compact: the same inside a toolbar or a table row.",
    "columns": "12",
    "wrap": "1880px",
    "page-pad": "clamp(24px, 2.6vw, 48px)",
    "cell-pad": "clamp(28px, 5.3vw, 76px)",
    "section-gap": "clamp(72px, 12.3vw, 176px)",
    "root-size": "clamp(100%, 11.5px + 0.3125vw, 125%)",
    "measure-lead": "36ch",
    "measure": "44ch",
    "measure-fine": "60ch",
    "family-bar": "2.375rem",
    "control-height": "44px",
    "control-height-compact": "32px"
  },
  "narrow": {
    "$description": "What changes under breakpoint.phone: the sheet meets the window, cells and sections tighten, body text is 1rem.",
    "page-pad": "0px",
    "cell-pad": "20px",
    "section-gap": "64px",
    "text-body": "1rem"
  },
  "breakpoint": {
    "$description": "Not emitted as custom properties (a media query cannot read one). phone: one column under it. mid: the main links fold into a menu and a quarter becomes a half under it.",
    "phone": "820px",
    "mid": "1100px"
  },
  "space": {
    "1": "0.25rem",
    "2": "0.5rem",
    "3": "0.75rem",
    "4": "1rem",
    "5": "1.5rem",
    "6": "2rem",
    "7": "3rem",
    "8": "4rem",
    "9": "6rem",
    "10": "8rem",
    "$description": "Spacing steps, the scale of the earlier tokens in rem: 4, 8, 12, 16, 24, 32, 48, 64, 96 and 128 px at a 16 px root."
  },
  "radius": {
    "$description": "control: buttons, fields, switches, menus and every other control. small: a bar on a time axis, a tag.",
    "control": "8px",
    "small": "3px"
  },
  "stroke": {
    "$description": "Line weights. hair: grid rules, borders. detail: what is painted on a face of a figure, a stitched seam. figure: the line of a figure; it does not thicken as the figure grows. mark: a link underline, the edge of a key or of an open label square. focus: the focus outline and the mark under the current item.",
    "hair": "1px",
    "detail": "0.9px",
    "figure": "1.3px",
    "mark": "1.5px",
    "focus": "2px"
  },
  "dash": {
    "$description": "Dash patterns of a figure, as stroke-dasharray. absent: not there yet, or not any more. seam: a stitched seam.",
    "absent": "4 3",
    "seam": "2 3"
  },
  "motion": {
    "$description": "duration-state: a hover, a pressed state. duration-lift: a figure that lifts under the pointer. duration-move: a part that moves when a state changes. duration-draw: the first figure drawing in once on load. The loops are the slow motion a drawing has while it is on screen: a coloured part breathes, a lamp blinks, a cursor blinks, a light runs along a bar. Everything stops under prefers-reduced-motion.",
    "ease": "cubic-bezier(0.2, 0.8, 0.2, 1)",
    "duration-state": "150ms",
    "duration-lift": "200ms",
    "duration-move": "500ms",
    "duration-draw": "1400ms",
    "loop-breathe": "6s",
    "loop-lamp": "3.6s",
    "loop-cursor": "1.15s",
    "loop-glint": "7s"
  }
};
export default line;
