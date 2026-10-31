use ::windows::Win32::UI::Accessibility::{
    UIA_AppBarControlTypeId, UIA_ButtonControlTypeId, UIA_CalendarControlTypeId,
    UIA_CheckBoxControlTypeId, UIA_ComboBoxControlTypeId, UIA_CONTROLTYPE_ID,
    UIA_CustomControlTypeId, UIA_DataGridControlTypeId, UIA_DataItemControlTypeId,
    UIA_DocumentControlTypeId, UIA_EditControlTypeId, UIA_GroupControlTypeId,
    UIA_HeaderControlTypeId, UIA_HeaderItemControlTypeId, UIA_HyperlinkControlTypeId,
    UIA_ImageControlTypeId, UIA_ListControlTypeId, UIA_ListItemControlTypeId,
    UIA_MenuBarControlTypeId, UIA_MenuControlTypeId, UIA_MenuItemControlTypeId,
    UIA_PaneControlTypeId, UIA_ProgressBarControlTypeId, UIA_RadioButtonControlTypeId,
    UIA_ScrollBarControlTypeId, UIA_SeparatorControlTypeId, UIA_SliderControlTypeId,
    UIA_SpinnerControlTypeId, UIA_SplitButtonControlTypeId, UIA_StatusBarControlTypeId,
    UIA_TabControlTypeId, UIA_TabItemControlTypeId, UIA_TableControlTypeId, UIA_TextControlTypeId,
    UIA_ThumbControlTypeId, UIA_TitleBarControlTypeId, UIA_ToolBarControlTypeId,
    UIA_ToolTipControlTypeId, UIA_TreeControlTypeId, UIA_TreeItemControlTypeId,
    UIA_WindowControlTypeId,
};

#[allow(non_upper_case_globals)]
pub fn role(control_type: UIA_CONTROLTYPE_ID) -> &'static str {
    match control_type {
        UIA_ButtonControlTypeId => "button",
        UIA_EditControlTypeId => "edit",
        UIA_CheckBoxControlTypeId => "checkbox",
        UIA_RadioButtonControlTypeId => "radio",
        UIA_ComboBoxControlTypeId => "combobox",
        UIA_TextControlTypeId => "text",
        UIA_ListControlTypeId => "list",
        UIA_ListItemControlTypeId => "listitem",
        UIA_DataItemControlTypeId => "listitem",
        UIA_TreeControlTypeId => "tree",
        UIA_TreeItemControlTypeId => "treeitem",
        UIA_TableControlTypeId => "table",
        UIA_DataGridControlTypeId => "table",
        UIA_HeaderControlTypeId => "header",
        UIA_HeaderItemControlTypeId => "headeritem",
        UIA_WindowControlTypeId => "window",
        UIA_PaneControlTypeId => "pane",
        UIA_DocumentControlTypeId => "document",
        UIA_GroupControlTypeId => "group",
        UIA_MenuControlTypeId => "menu",
        UIA_MenuBarControlTypeId => "menu",
        UIA_MenuItemControlTypeId => "menuitem",
        UIA_TabControlTypeId => "tab",
        UIA_TabItemControlTypeId => "tabitem",
        UIA_SliderControlTypeId => "slider",
        UIA_SpinnerControlTypeId => "spinner",
        UIA_ProgressBarControlTypeId => "progressbar",
        UIA_HyperlinkControlTypeId => "hyperlink",
        UIA_ImageControlTypeId => "image",
        UIA_ToolBarControlTypeId => "toolbar",
        UIA_ToolTipControlTypeId => "tooltip",
        UIA_ScrollBarControlTypeId => "scrollbar",
        UIA_StatusBarControlTypeId => "statusbar",
        UIA_TitleBarControlTypeId => "titlebar",
        UIA_SplitButtonControlTypeId => "splitbutton",
        UIA_CalendarControlTypeId => "calendar",
        UIA_SeparatorControlTypeId => "separator",
        UIA_ThumbControlTypeId => "thumb",
        UIA_AppBarControlTypeId => "appbar",
        UIA_CustomControlTypeId => "custom",
        _ => "unknown",
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn maps_common_control_types() {
        assert_eq!(role(UIA_ButtonControlTypeId), "button");
        assert_eq!(role(UIA_EditControlTypeId), "edit");
        assert_eq!(role(UIA_CheckBoxControlTypeId), "checkbox");
        assert_eq!(role(UIA_CONTROLTYPE_ID(99999)), "unknown");
    }
}
