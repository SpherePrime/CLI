use super::panel::Bounds;

pub const OFFSCREEN_OFFSET: f64 = 40000.0;

#[derive(Clone, Copy, Debug, PartialEq)]
pub struct TabPlacement {
    pub visible: bool,
    pub x: f64,
    pub y: f64,
    pub width: f64,
    pub height: f64,
}

pub fn fits_panel(bounds: &Bounds, viewport: Option<f64>) -> bool {
    match viewport {
        Some(w) if bounds.width > 0.0 && w.round() > bounds.width.round() => false,
        _ => true,
    }
}

pub fn compute_placement(
    shown: bool,
    offscreen: bool,
    bounds: &Bounds,
    viewport: Option<f64>,
) -> TabPlacement {
    let offscreen_height = bounds.height.max(720.0).round().max(1.0);
    let offscreen_width = viewport
        .unwrap_or_else(|| {
            if bounds.width > 0.0 {
                bounds.width
            } else {
                1280.0
            }
        })
        .round()
        .max(1.0);
    if shown && fits_panel(bounds, viewport) {
        let width = viewport.unwrap_or(bounds.width).round().max(1.0);
        let x = (bounds.x + ((bounds.width - width).max(0.0) / 2.0)).round();
        let y = bounds.y.round();
        let height = bounds.height.round().max(1.0);
        TabPlacement {
            visible: true,
            x,
            y,
            width,
            height,
        }
    } else {
        TabPlacement {
            visible: offscreen,
            x: -OFFSCREEN_OFFSET,
            y: 0.0,
            width: offscreen_width,
            height: offscreen_height,
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn fits_panel_checks_viewport_against_bounds() {
        let bounds = Bounds {
            x: 0.0,
            y: 0.0,
            width: 800.0,
            height: 600.0,
        };
        assert!(fits_panel(&bounds, None));
        assert!(fits_panel(&bounds, Some(600.0)));
        assert!(fits_panel(&bounds, Some(800.0)));
        assert!(fits_panel(&bounds, Some(800.4)));
        assert!(!fits_panel(&bounds, Some(800.6)));
        assert!(!fits_panel(&bounds, Some(1024.0)));

        let zero_bounds = Bounds {
            x: 0.0,
            y: 0.0,
            width: 0.0,
            height: 0.0,
        };
        assert!(fits_panel(&zero_bounds, None));
        assert!(fits_panel(&zero_bounds, Some(1440.0)));
    }

    #[test]
    fn hidden_tabs_park_offscreen_and_invisible() {
        let bounds = Bounds {
            x: 100.0,
            y: 50.0,
            width: 800.0,
            height: 600.0,
        };
        let placement = compute_placement(false, false, &bounds, None);
        assert_eq!(
            placement,
            TabPlacement {
                visible: false,
                x: -OFFSCREEN_OFFSET,
                y: 0.0,
                width: 800.0,
                height: 720.0,
            }
        );
        assert_ne!(placement.x, bounds.x);
        assert_ne!(placement.y, bounds.y);
    }

    #[test]
    fn offscreen_rendering_tabs_render_at_offscreen_offset_with_requested_width() {
        let bounds = Bounds {
            x: 200.0,
            y: 100.0,
            width: 500.0,
            height: 400.0,
        };
        let placement = compute_placement(false, true, &bounds, Some(1440.0));
        assert_eq!(
            placement,
            TabPlacement {
                visible: true,
                x: -OFFSCREEN_OFFSET,
                y: 0.0,
                width: 1440.0,
                height: 720.0,
            }
        );
    }

    #[test]
    fn shown_tab_with_no_viewport_fits_inside_panel_bounds() {
        let bounds = Bounds {
            x: 250.0,
            y: 80.0,
            width: 600.0,
            height: 800.0,
        };
        let placement = compute_placement(true, false, &bounds, None);
        assert_eq!(
            placement,
            TabPlacement {
                visible: true,
                x: 250.0,
                y: 80.0,
                width: 600.0,
                height: 800.0,
            }
        );
    }

    #[test]
    fn shown_tab_with_narrower_viewport_is_centered_horizontally() {
        let bounds = Bounds {
            x: 100.0,
            y: 50.0,
            width: 800.0,
            height: 600.0,
        };
        let placement = compute_placement(true, false, &bounds, Some(400.0));
        assert_eq!(
            placement,
            TabPlacement {
                visible: true,
                x: 300.0,
                y: 50.0,
                width: 400.0,
                height: 600.0,
            }
        );
    }

    #[test]
    fn shown_tab_wider_than_bounds_moves_offscreen() {
        let bounds = Bounds {
            x: 100.0,
            y: 50.0,
            width: 600.0,
            height: 800.0,
        };
        let placement_idle = compute_placement(true, false, &bounds, Some(1200.0));
        assert_eq!(
            placement_idle,
            TabPlacement {
                visible: false,
                x: -OFFSCREEN_OFFSET,
                y: 0.0,
                width: 1200.0,
                height: 800.0,
            }
        );
        let placement_rendering = compute_placement(true, true, &bounds, Some(1200.0));
        assert_eq!(
            placement_rendering,
            TabPlacement {
                visible: true,
                x: -OFFSCREEN_OFFSET,
                y: 0.0,
                width: 1200.0,
                height: 800.0,
            }
        );
    }
}
