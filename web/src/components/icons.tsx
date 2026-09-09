// Icon compatibility layer.
//
// The dashboard was written against lucide-react icon names. This module keeps
// those names as the public API but renders HugeIcons (the icon set used by the
// Fullpilot dashboard) underneath, so call sites do not change.
//
// Every export keeps lucide-compatible props: className, size, strokeWidth,
// style, aria-*, onClick, ref. Size is driven by the className (Tailwind
// `w-3.5 h-3.5` etc.) unless an explicit `size` is given. Stroke width
// defaults to 1.75.
//
// Generated mapping: 233 icons render HugeIcons, 3 fall back
// to lucide because HugeIcons has no free equivalent (HistoryIcon, ShieldAlertIcon, ShieldQuestionIcon).

import { forwardRef } from "react";
import type { ForwardRefExoticComponent, RefAttributes, SVGProps } from "react";
import { HugeiconsIcon } from "@hugeicons/react";
import type { IconSvgElement } from "@hugeicons/react";
import {
    Activity01Icon as H_Activity01Icon,
    Agreement01Icon as H_Agreement01Icon,
    Alert02Icon as H_Alert02Icon,
    AlertCircleIcon as H_AlertCircleIcon,
    AlertDiamondIcon as H_AlertDiamondIcon,
    Archive02Icon as H_Archive02Icon,
    ArrowDown01Icon as H_ArrowDown01Icon,
    ArrowDown02Icon as H_ArrowDown02Icon,
    ArrowExpand01Icon as H_ArrowExpand01Icon,
    ArrowLeft01Icon as H_ArrowLeft01Icon,
    ArrowLeft02Icon as H_ArrowLeft02Icon,
    ArrowLeftRightIcon as H_ArrowLeftRightIcon,
    ArrowRight01Icon as H_ArrowRight01Icon,
    ArrowRight02Icon as H_ArrowRight02Icon,
    ArrowShrink01Icon as H_ArrowShrink01Icon,
    ArrowTurnBackwardIcon as H_ArrowTurnBackwardIcon,
    ArrowTurnForwardIcon as H_ArrowTurnForwardIcon,
    ArrowUp01Icon as H_ArrowUp01Icon,
    ArrowUp02Icon as H_ArrowUp02Icon,
    ArrowUpDownIcon as H_ArrowUpDownIcon,
    ArrowUpRight01Icon as H_ArrowUpRight01Icon,
    AtIcon as H_AtIcon,
    Attachment01Icon as H_Attachment01Icon,
    BarChartIcon as H_BarChartIcon,
    BookmarkAdd02Icon as H_BookmarkAdd02Icon,
    Briefcase01Icon as H_Briefcase01Icon,
    BubbleChatQuestionIcon as H_BubbleChatQuestionIcon,
    Building02Icon as H_Building02Icon,
    Building03Icon as H_Building03Icon,
    Calendar01Icon as H_Calendar01Icon,
    Calendar03Icon as H_Calendar03Icon,
    CalendarAdd01Icon as H_CalendarAdd01Icon,
    CalendarCheckIn01Icon as H_CalendarCheckIn01Icon,
    CalendarRemove01Icon as H_CalendarRemove01Icon,
    CallIcon as H_CallIcon,
    Camera01Icon as H_Camera01Icon,
    Cancel01Icon as H_Cancel01Icon,
    CancelCircleIcon as H_CancelCircleIcon,
    CancelSquareIcon as H_CancelSquareIcon,
    ChampionIcon as H_ChampionIcon,
    ChartColumnIcon as H_ChartColumnIcon,
    CheckListIcon as H_CheckListIcon,
    CheckmarkCircle02Icon as H_CheckmarkCircle02Icon,
    CheckmarkSquare02Icon as H_CheckmarkSquare02Icon,
    CircleArrowUp02Icon as H_CircleArrowUp02Icon,
    CircleIcon as H_CircleIcon,
    ClipboardIcon as H_ClipboardIcon,
    Clock01Icon as H_Clock01Icon,
    Clock02Icon as H_Clock02Icon,
    CloudIcon as H_CloudIcon,
    CloudUploadIcon as H_CloudUploadIcon,
    Coffee02Icon as H_Coffee02Icon,
    Coins01Icon as H_Coins01Icon,
    ColorPickerIcon as H_ColorPickerIcon,
    CommandLineIcon as H_CommandLineIcon,
    Comment01Icon as H_Comment01Icon,
    ComputerIcon as H_ComputerIcon,
    Copy01Icon as H_Copy01Icon,
    CreditCardIcon as H_CreditCardIcon,
    CubeIcon as H_CubeIcon,
    CursorPointer01Icon as H_CursorPointer01Icon,
    DashboardSpeed01Icon as H_DashboardSpeed01Icon,
    DashboardSquare01Icon as H_DashboardSquare01Icon,
    DashedLineCircleIcon as H_DashedLineCircleIcon,
    Database01Icon as H_Database01Icon,
    Delete02Icon as H_Delete02Icon,
    DollarCircleIcon as H_DollarCircleIcon,
    Download04Icon as H_Download04Icon,
    DragDropVerticalIcon as H_DragDropVerticalIcon,
    EyeIcon as H_EyeIcon,
    File01Icon as H_File01Icon,
    File02Icon as H_File02Icon,
    FileScriptIcon as H_FileScriptIcon,
    FileZipIcon as H_FileZipIcon,
    FilterIcon as H_FilterIcon,
    FilterRemoveIcon as H_FilterRemoveIcon,
    FireIcon as H_FireIcon,
    Flag02Icon as H_Flag02Icon,
    FloppyDiskIcon as H_FloppyDiskIcon,
    Folder01Icon as H_Folder01Icon,
    FunctionSquareIcon as H_FunctionSquareIcon,
    GiftIcon as H_GiftIcon,
    GitBranchIcon as H_GitBranchIcon,
    GitForkIcon as H_GitForkIcon,
    Globe02Icon as H_Globe02Icon,
    GoogleSheetIcon as H_GoogleSheetIcon,
    GridViewIcon as H_GridViewIcon,
    Heading02Icon as H_Heading02Icon,
    HeadingIcon as H_HeadingIcon,
    HelpCircleIcon as H_HelpCircleIcon,
    HierarchyIcon as H_HierarchyIcon,
    HierarchySquare01Icon as H_HierarchySquare01Icon,
    HourglassIcon as H_HourglassIcon,
    Image01Icon as H_Image01Icon,
    InboxIcon as H_InboxIcon,
    InformationCircleIcon as H_InformationCircleIcon,
    Key01Icon as H_Key01Icon,
    Layers01Icon as H_Layers01Icon,
    LeftToRightListBulletIcon as H_LeftToRightListBulletIcon,
    LeftToRightListNumberIcon as H_LeftToRightListNumberIcon,
    Link01Icon as H_Link01Icon,
    Link04Icon as H_Link04Icon,
    LinkSquare02Icon as H_LinkSquare02Icon,
    ListViewIcon as H_ListViewIcon,
    Loading02Icon as H_Loading02Icon,
    Loading03Icon as H_Loading03Icon,
    LockIcon as H_LockIcon,
    Login03Icon as H_Login03Icon,
    Logout03Icon as H_Logout03Icon,
    MagicWand01Icon as H_MagicWand01Icon,
    Mail01Icon as H_Mail01Icon,
    MailAdd01Icon as H_MailAdd01Icon,
    MailBlock01Icon as H_MailBlock01Icon,
    MailOpen01Icon as H_MailOpen01Icon,
    MailRemove01Icon as H_MailRemove01Icon,
    MailSearch01Icon as H_MailSearch01Icon,
    MailValidation01Icon as H_MailValidation01Icon,
    Mailbox01Icon as H_Mailbox01Icon,
    Menu01Icon as H_Menu01Icon,
    Megaphone01Icon as H_Megaphone01Icon,
    MessageMultiple01Icon as H_MessageMultiple01Icon,
    MinusSignIcon as H_MinusSignIcon,
    Moon02Icon as H_Moon02Icon,
    MoreHorizontalIcon as H_MoreHorizontalIcon,
    MoreVerticalIcon as H_MoreVerticalIcon,
    Notification03Icon as H_Notification03Icon,
    NotificationOff03Icon as H_NotificationOff03Icon,
    PaintBoardIcon as H_PaintBoardIcon,
    PauseCircleIcon as H_PauseCircleIcon,
    PauseIcon as H_PauseIcon,
    PencilEdit01Icon as H_PencilEdit01Icon,
    PencilIcon as H_PencilIcon,
    PictureInPictureOnIcon as H_PictureInPictureOnIcon,
    PlayIcon as H_PlayIcon,
    Plug01Icon as H_Plug01Icon,
    Plug02Icon as H_Plug02Icon,
    PlugSocketIcon as H_PlugSocketIcon,
    PlusSignIcon as H_PlusSignIcon,
    RadioButtonIcon as H_RadioButtonIcon,
    RefreshIcon as H_RefreshIcon,
    Rocket01Icon as H_Rocket01Icon,
    Rotate01Icon as H_Rotate01Icon,
    Rotate02Icon as H_Rotate02Icon,
    Search01Icon as H_Search01Icon,
    SearchRemoveIcon as H_SearchRemoveIcon,
    SecurityBlockIcon as H_SecurityBlockIcon,
    SecurityCheckIcon as H_SecurityCheckIcon,
    SentIcon as H_SentIcon,
    ServerStack01Icon as H_ServerStack01Icon,
    ServerStack02Icon as H_ServerStack02Icon,
    Settings02Icon as H_Settings02Icon,
    Share08Icon as H_Share08Icon,
    Shield01Icon as H_Shield01Icon,
    ShuffleIcon as H_ShuffleIcon,
    SidebarLeftIcon as H_SidebarLeftIcon,
    SidebarRightIcon as H_SidebarRightIcon,
    SlidersHorizontalIcon as H_SlidersHorizontalIcon,
    SlidersVerticalIcon as H_SlidersVerticalIcon,
    SmartPhone01Icon as H_SmartPhone01Icon,
    SmileIcon as H_SmileIcon,
    SolidLine01Icon as H_SolidLine01Icon,
    SourceCodeIcon as H_SourceCodeIcon,
    SparklesIcon as H_SparklesIcon,
    SquareArrowDown01Icon as H_SquareArrowDown01Icon,
    SquareIcon as H_SquareIcon,
    StickyNote02Icon as H_StickyNote02Icon,
    Sun03Icon as H_Sun03Icon,
    SunriseIcon as H_SunriseIcon,
    SunsetIcon as H_SunsetIcon,
    Table01Icon as H_Table01Icon,
    Tag01Icon as H_Tag01Icon,
    TagsIcon as H_TagsIcon,
    TextAlignLeftIcon as H_TextAlignLeftIcon,
    TextBoldIcon as H_TextBoldIcon,
    TextCheckIcon as H_TextCheckIcon,
    TextFontIcon as H_TextFontIcon,
    TextIcon as H_TextIcon,
    TextItalicIcon as H_TextItalicIcon,
    TextNumberSignIcon as H_TextNumberSignIcon,
    TextStrikethroughIcon as H_TextStrikethroughIcon,
    TextUnderlineIcon as H_TextUnderlineIcon,
    ThumbsDownIcon as H_ThumbsDownIcon,
    ThumbsUpIcon as H_ThumbsUpIcon,
    Tick02Icon as H_Tick02Icon,
    Ticket01Icon as H_Ticket01Icon,
    TimeScheduleIcon as H_TimeScheduleIcon,
    Timer01Icon as H_Timer01Icon,
    ToolsIcon as H_ToolsIcon,
    UnavailableIcon as H_UnavailableIcon,
    Undo02Icon as H_Undo02Icon,
    UndoIcon as H_UndoIcon,
    Unlink01Icon as H_Unlink01Icon,
    Unlink02Icon as H_Unlink02Icon,
    Upload04Icon as H_Upload04Icon,
    UserAdd01Icon as H_UserAdd01Icon,
    UserIcon as H_UserIcon,
    UserMinus01Icon as H_UserMinus01Icon,
    UserMultiple02Icon as H_UserMultiple02Icon,
    UserMultipleIcon as H_UserMultipleIcon,
    UserRemove01Icon as H_UserRemove01Icon,
    Video01Icon as H_Video01Icon,
    ViewOffSlashIcon as H_ViewOffSlashIcon,
    WebhookIcon as H_WebhookIcon,
    Wrench01Icon as H_Wrench01Icon,
    Xls01Icon as H_Xls01Icon,
    ZapIcon as H_ZapIcon,
} from "@hugeicons/core-free-icons";
import {
    HistoryIcon as LucideHistoryIcon,
    ShieldAlertIcon as LucideShieldAlertIcon,
    ShieldQuestionIcon as LucideShieldQuestionIcon,
} from "lucide-react";

export type IconProps = Omit<SVGProps<SVGSVGElement>, "ref" | "size"> & {
    size?: number | string;
    strokeWidth?: number;
    absoluteStrokeWidth?: boolean;
};

export type IconComponent = ForwardRefExoticComponent<IconProps & RefAttributes<SVGSVGElement>>;

/** Alias so `import type { LucideIcon }` keeps working against this layer. */
export type LucideIcon = IconComponent;
export type LucideProps = IconProps;

const DEFAULT_STROKE_WIDTH = 1.75;

function createIcon(displayName: string, icon: IconSvgElement): IconComponent {
    const Icon = forwardRef<SVGSVGElement, IconProps>(function Icon(
        { strokeWidth = DEFAULT_STROKE_WIDTH, size, ...rest },
        ref,
    ) {
        return (
            <HugeiconsIcon
                ref={ref}
                icon={icon}
                strokeWidth={strokeWidth}
                {...(size !== undefined ? { size } : {})}
                {...rest}
            />
        );
    });
    Icon.displayName = displayName;
    return Icon;
}

export const ActivityIcon = createIcon("ActivityIcon", H_Activity01Icon);
export const AlertCircleIcon = createIcon("AlertCircleIcon", H_AlertCircleIcon);
export const AlertOctagonIcon = createIcon("AlertOctagonIcon", H_AlertDiamondIcon);
export const AlertTriangle = createIcon("AlertTriangle", H_Alert02Icon);
export const AlertTriangleIcon = createIcon("AlertTriangleIcon", H_Alert02Icon);
export const AlignLeftIcon = createIcon("AlignLeftIcon", H_TextAlignLeftIcon);
export const ArchiveIcon = createIcon("ArchiveIcon", H_Archive02Icon);
export const ArrowDownIcon = createIcon("ArrowDownIcon", H_ArrowDown02Icon);
export const ArrowLeft = createIcon("ArrowLeft", H_ArrowLeft02Icon);
export const ArrowLeftIcon = createIcon("ArrowLeftIcon", H_ArrowLeft02Icon);
export const ArrowRight = createIcon("ArrowRight", H_ArrowRight02Icon);
export const ArrowRightIcon = createIcon("ArrowRightIcon", H_ArrowRight02Icon);
export const ArrowRightLeftIcon = createIcon("ArrowRightLeftIcon", H_ArrowLeftRightIcon);
export const ArrowUpCircleIcon = createIcon("ArrowUpCircleIcon", H_CircleArrowUp02Icon);
export const ArrowUpDownIcon = createIcon("ArrowUpDownIcon", H_ArrowUpDownIcon);
export const ArrowUpIcon = createIcon("ArrowUpIcon", H_ArrowUp02Icon);
export const ArrowUpRightIcon = createIcon("ArrowUpRightIcon", H_ArrowUpRight01Icon);
export const AtSignIcon = createIcon("AtSignIcon", H_AtIcon);
export const BanIcon = createIcon("BanIcon", H_UnavailableIcon);
export const BarChart3Icon = createIcon("BarChart3Icon", H_BarChartIcon);
export const BellIcon = createIcon("BellIcon", H_Notification03Icon);
export const BellOffIcon = createIcon("BellOffIcon", H_NotificationOff03Icon);
export const BoldIcon = createIcon("BoldIcon", H_TextBoldIcon);
export const BookmarkPlusIcon = createIcon("BookmarkPlusIcon", H_BookmarkAdd02Icon);
export const BoxesIcon = createIcon("BoxesIcon", H_CubeIcon);
export const BracesIcon = createIcon("BracesIcon", H_SourceCodeIcon);
export const BriefcaseIcon = createIcon("BriefcaseIcon", H_Briefcase01Icon);
export const Building2Icon = createIcon("Building2Icon", H_Building03Icon);
export const BuildingIcon = createIcon("BuildingIcon", H_Building02Icon);
export const CableIcon = createIcon("CableIcon", H_PlugSocketIcon);
export const CalendarCheckIcon = createIcon("CalendarCheckIcon", H_CalendarCheckIn01Icon);
export const CalendarClockIcon = createIcon("CalendarClockIcon", H_TimeScheduleIcon);
export const CalendarIcon = createIcon("CalendarIcon", H_Calendar03Icon);
export const CalendarPlusIcon = createIcon("CalendarPlusIcon", H_CalendarAdd01Icon);
export const CalendarRangeIcon = createIcon("CalendarRangeIcon", H_Calendar01Icon);
export const CalendarXIcon = createIcon("CalendarXIcon", H_CalendarRemove01Icon);
export const CameraIcon = createIcon("CameraIcon", H_Camera01Icon);
export const ChartNoAxesColumnIcon = createIcon("ChartNoAxesColumnIcon", H_ChartColumnIcon);
export const Check = createIcon("Check", H_Tick02Icon);
export const CheckCircle2Icon = createIcon("CheckCircle2Icon", H_CheckmarkCircle02Icon);
export const CheckIcon = createIcon("CheckIcon", H_Tick02Icon);
export const CheckSquareIcon = createIcon("CheckSquareIcon", H_CheckmarkSquare02Icon);
export const ChevronDownIcon = createIcon("ChevronDownIcon", H_ArrowDown01Icon);
export const ChevronDownSquareIcon = createIcon("ChevronDownSquareIcon", H_SquareArrowDown01Icon);
export const ChevronLeftIcon = createIcon("ChevronLeftIcon", H_ArrowLeft01Icon);
export const ChevronRight = createIcon("ChevronRight", H_ArrowRight01Icon);
export const ChevronRightIcon = createIcon("ChevronRightIcon", H_ArrowRight01Icon);
export const ChevronUpIcon = createIcon("ChevronUpIcon", H_ArrowUp01Icon);
export const CircleCheckIcon = createIcon("CircleCheckIcon", H_CheckmarkCircle02Icon);
export const CircleDashedIcon = createIcon("CircleDashedIcon", H_DashedLineCircleIcon);
export const CircleDollarSignIcon = createIcon("CircleDollarSignIcon", H_DollarCircleIcon);
export const CircleDotIcon = createIcon("CircleDotIcon", H_RadioButtonIcon);
export const CircleHelpIcon = createIcon("CircleHelpIcon", H_HelpCircleIcon);
export const CircleIcon = createIcon("CircleIcon", H_CircleIcon);
export const ClipboardListIcon = createIcon("ClipboardListIcon", H_ClipboardIcon);
export const ClockFadingIcon = createIcon("ClockFadingIcon", H_Clock02Icon);
export const ClockIcon = createIcon("ClockIcon", H_Clock01Icon);
export const CloudIcon = createIcon("CloudIcon", H_CloudIcon);
export const CoffeeIcon = createIcon("CoffeeIcon", H_Coffee02Icon);
export const CoinsIcon = createIcon("CoinsIcon", H_Coins01Icon);
export const CopyIcon = createIcon("CopyIcon", H_Copy01Icon);
export const CornerUpLeftIcon = createIcon("CornerUpLeftIcon", H_ArrowTurnBackwardIcon);
export const CreditCardIcon = createIcon("CreditCardIcon", H_CreditCardIcon);
export const DatabaseIcon = createIcon("DatabaseIcon", H_Database01Icon);
export const DownloadIcon = createIcon("DownloadIcon", H_Download04Icon);
export const ExternalLinkIcon = createIcon("ExternalLinkIcon", H_LinkSquare02Icon);
export const EyeIcon = createIcon("EyeIcon", H_EyeIcon);
export const EyeOffIcon = createIcon("EyeOffIcon", H_ViewOffSlashIcon);
export const FileArchiveIcon = createIcon("FileArchiveIcon", H_FileZipIcon);
export const FileIcon = createIcon("FileIcon", H_File01Icon);
export const FileJsonIcon = createIcon("FileJsonIcon", H_FileScriptIcon);
export const FileSpreadsheetIcon = createIcon("FileSpreadsheetIcon", H_Xls01Icon);
export const FileTextIcon = createIcon("FileTextIcon", H_File02Icon);
export const FilterIcon = createIcon("FilterIcon", H_FilterIcon);
export const FilterXIcon = createIcon("FilterXIcon", H_FilterRemoveIcon);
export const FlagIcon = createIcon("FlagIcon", H_Flag02Icon);
export const FlameIcon = createIcon("FlameIcon", H_FireIcon);
export const FolderIcon = createIcon("FolderIcon", H_Folder01Icon);
export const ForwardIcon = createIcon("ForwardIcon", H_ArrowTurnForwardIcon);
export const FunctionSquareIcon = createIcon("FunctionSquareIcon", H_FunctionSquareIcon);
export const GaugeIcon = createIcon("GaugeIcon", H_DashboardSpeed01Icon);
export const GiftIcon = createIcon("GiftIcon", H_GiftIcon);
export const GitBranchIcon = createIcon("GitBranchIcon", H_GitBranchIcon);
export const Globe = createIcon("Globe", H_Globe02Icon);
export const GlobeIcon = createIcon("GlobeIcon", H_Globe02Icon);
export const GripVerticalIcon = createIcon("GripVerticalIcon", H_DragDropVerticalIcon);
export const HammerIcon = createIcon("HammerIcon", H_ToolsIcon);
export const HandshakeIcon = createIcon("HandshakeIcon", H_Agreement01Icon);
export const HashIcon = createIcon("HashIcon", H_TextNumberSignIcon);
export const Heading2Icon = createIcon("Heading2Icon", H_Heading02Icon);
export const HeadingIcon = createIcon("HeadingIcon", H_HeadingIcon);
export const HourglassIcon = createIcon("HourglassIcon", H_HourglassIcon);
export const ImageIcon = createIcon("ImageIcon", H_Image01Icon);
export const InboxIcon = createIcon("InboxIcon", H_InboxIcon);
export const InfoIcon = createIcon("InfoIcon", H_InformationCircleIcon);
export const ItalicIcon = createIcon("ItalicIcon", H_TextItalicIcon);
export const KeyIcon = createIcon("KeyIcon", H_Key01Icon);
export const KeyRound = createIcon("KeyRound", H_Key01Icon);
export const KeyRoundIcon = createIcon("KeyRoundIcon", H_Key01Icon);
export const LayersIcon = createIcon("LayersIcon", H_Layers01Icon);
export const LayoutDashboard = createIcon("LayoutDashboard", H_DashboardSquare01Icon);
export const LayoutGridIcon = createIcon("LayoutGridIcon", H_GridViewIcon);
export const LayoutListIcon = createIcon("LayoutListIcon", H_ListViewIcon);
export const Link2Icon = createIcon("Link2Icon", H_Link04Icon);
export const LinkIcon = createIcon("LinkIcon", H_Link01Icon);
export const ListChecksIcon = createIcon("ListChecksIcon", H_CheckListIcon);
export const ListIcon = createIcon("ListIcon", H_LeftToRightListBulletIcon);
export const ListOrderedIcon = createIcon("ListOrderedIcon", H_LeftToRightListNumberIcon);
export const ListTreeIcon = createIcon("ListTreeIcon", H_HierarchyIcon);
export const Loader2Icon = createIcon("Loader2Icon", H_Loading03Icon);
export const LoaderIcon = createIcon("LoaderIcon", H_Loading02Icon);
export const LockIcon = createIcon("LockIcon", H_LockIcon);
export const LogIn = createIcon("LogIn", H_Login03Icon);
export const LogOut = createIcon("LogOut", H_Logout03Icon);
export const LogOutIcon = createIcon("LogOutIcon", H_Logout03Icon);
export const Mail = createIcon("Mail", H_Mail01Icon);
export const MailCheckIcon = createIcon("MailCheckIcon", H_MailValidation01Icon);
export const MailIcon = createIcon("MailIcon", H_Mail01Icon);
export const MailOpenIcon = createIcon("MailOpenIcon", H_MailOpen01Icon);
export const MailPlusIcon = createIcon("MailPlusIcon", H_MailAdd01Icon);
export const MailQuestionIcon = createIcon("MailQuestionIcon", H_MailSearch01Icon);
export const MailWarningIcon = createIcon("MailWarningIcon", H_MailBlock01Icon);
export const MailX = createIcon("MailX", H_MailRemove01Icon);
export const MailboxIcon = createIcon("MailboxIcon", H_Mailbox01Icon);
export const MenuIcon = createIcon("MenuIcon", H_Menu01Icon);
export const Maximize2Icon = createIcon("Maximize2Icon", H_ArrowExpand01Icon);
export const MegaphoneIcon = createIcon("MegaphoneIcon", H_Megaphone01Icon);
export const MessageCircleQuestionIcon = createIcon("MessageCircleQuestionIcon", H_BubbleChatQuestionIcon);
export const MessageSquareIcon = createIcon("MessageSquareIcon", H_Comment01Icon);
export const MessagesSquareIcon = createIcon("MessagesSquareIcon", H_MessageMultiple01Icon);
export const Minimize2Icon = createIcon("Minimize2Icon", H_ArrowShrink01Icon);
export const MinusIcon = createIcon("MinusIcon", H_MinusSignIcon);
export const Monitor = createIcon("Monitor", H_ComputerIcon);
export const MonitorIcon = createIcon("MonitorIcon", H_ComputerIcon);
export const MoonIcon = createIcon("MoonIcon", H_Moon02Icon);
export const MoreHorizontal = createIcon("MoreHorizontal", H_MoreHorizontalIcon);
export const MoreHorizontalIcon = createIcon("MoreHorizontalIcon", H_MoreHorizontalIcon);
export const MoreVerticalIcon = createIcon("MoreVerticalIcon", H_MoreVerticalIcon);
export const MousePointerClickIcon = createIcon("MousePointerClickIcon", H_CursorPointer01Icon);
export const NetworkIcon = createIcon("NetworkIcon", H_HierarchySquare01Icon);
export const OctagonAlertIcon = createIcon("OctagonAlertIcon", H_AlertDiamondIcon);
export const OctagonXIcon = createIcon("OctagonXIcon", H_CancelSquareIcon);
export const PaletteIcon = createIcon("PaletteIcon", H_PaintBoardIcon);
export const PanelLeftIcon = createIcon("PanelLeftIcon", H_SidebarLeftIcon);
export const PanelRightIcon = createIcon("PanelRightIcon", H_SidebarRightIcon);
export const PaperclipIcon = createIcon("PaperclipIcon", H_Attachment01Icon);
export const PauseCircleIcon = createIcon("PauseCircleIcon", H_PauseCircleIcon);
export const PauseIcon = createIcon("PauseIcon", H_PauseIcon);
export const PenLineIcon = createIcon("PenLineIcon", H_PencilEdit01Icon);
export const Pencil = createIcon("Pencil", H_PencilIcon);
export const PencilIcon = createIcon("PencilIcon", H_PencilIcon);
export const PencilLineIcon = createIcon("PencilLineIcon", H_PencilEdit01Icon);
export const PhoneIcon = createIcon("PhoneIcon", H_CallIcon);
export const PictureInPicture2Icon = createIcon("PictureInPicture2Icon", H_PictureInPictureOnIcon);
export const PipetteIcon = createIcon("PipetteIcon", H_ColorPickerIcon);
export const PlayIcon = createIcon("PlayIcon", H_PlayIcon);
export const PlugIcon = createIcon("PlugIcon", H_Plug01Icon);
export const PlugZapIcon = createIcon("PlugZapIcon", H_Plug02Icon);
export const Plus = createIcon("Plus", H_PlusSignIcon);
export const PlusIcon = createIcon("PlusIcon", H_PlusSignIcon);
export const RefreshCcwIcon = createIcon("RefreshCcwIcon", H_RefreshIcon);
export const RefreshCwIcon = createIcon("RefreshCwIcon", H_RefreshIcon);
export const ReplyIcon = createIcon("ReplyIcon", H_ArrowTurnBackwardIcon);
export const RocketIcon = createIcon("RocketIcon", H_Rocket01Icon);
export const RotateCcwIcon = createIcon("RotateCcwIcon", H_Rotate02Icon);
export const RotateCwIcon = createIcon("RotateCwIcon", H_Rotate01Icon);
export const SaveIcon = createIcon("SaveIcon", H_FloppyDiskIcon);
export const Search = createIcon("Search", H_Search01Icon);
export const SearchIcon = createIcon("SearchIcon", H_Search01Icon);
export const SearchXIcon = createIcon("SearchXIcon", H_SearchRemoveIcon);
export const SendIcon = createIcon("SendIcon", H_SentIcon);
export const SeparatorHorizontalIcon = createIcon("SeparatorHorizontalIcon", H_SolidLine01Icon);
export const ServerCrashIcon = createIcon("ServerCrashIcon", H_ServerStack02Icon);
export const ServerIcon = createIcon("ServerIcon", H_ServerStack01Icon);
export const Settings2Icon = createIcon("Settings2Icon", H_SlidersHorizontalIcon);
export const SettingsIcon = createIcon("SettingsIcon", H_Settings02Icon);
export const Share2Icon = createIcon("Share2Icon", H_Share08Icon);
export const SheetIcon = createIcon("SheetIcon", H_GoogleSheetIcon);
export const ShieldCheckIcon = createIcon("ShieldCheckIcon", H_SecurityCheckIcon);
export const ShieldIcon = createIcon("ShieldIcon", H_Shield01Icon);
export const ShieldOffIcon = createIcon("ShieldOffIcon", H_SecurityBlockIcon);
export const ShieldXIcon = createIcon("ShieldXIcon", H_SecurityBlockIcon);
export const ShuffleIcon = createIcon("ShuffleIcon", H_ShuffleIcon);
export const SlidersHorizontalIcon = createIcon("SlidersHorizontalIcon", H_SlidersHorizontalIcon);
export const SlidersIcon = createIcon("SlidersIcon", H_SlidersVerticalIcon);
export const Smartphone = createIcon("Smartphone", H_SmartPhone01Icon);
export const SmileIcon = createIcon("SmileIcon", H_SmileIcon);
export const SparkleIcon = createIcon("SparkleIcon", H_SparklesIcon);
export const SparklesIcon = createIcon("SparklesIcon", H_SparklesIcon);
export const SpellCheckIcon = createIcon("SpellCheckIcon", H_TextCheckIcon);
export const SplitIcon = createIcon("SplitIcon", H_GitForkIcon);
export const SquareIcon = createIcon("SquareIcon", H_SquareIcon);
export const StickyNoteIcon = createIcon("StickyNoteIcon", H_StickyNote02Icon);
export const StrikethroughIcon = createIcon("StrikethroughIcon", H_TextStrikethroughIcon);
export const SunIcon = createIcon("SunIcon", H_Sun03Icon);
export const SunriseIcon = createIcon("SunriseIcon", H_SunriseIcon);
export const SunsetIcon = createIcon("SunsetIcon", H_SunsetIcon);
export const Table2Icon = createIcon("Table2Icon", H_Table01Icon);
export const TagIcon = createIcon("TagIcon", H_Tag01Icon);
export const TagsIcon = createIcon("TagsIcon", H_TagsIcon);
export const TerminalIcon = createIcon("TerminalIcon", H_CommandLineIcon);
export const TextIcon = createIcon("TextIcon", H_TextIcon);
export const ThumbsDownIcon = createIcon("ThumbsDownIcon", H_ThumbsDownIcon);
export const ThumbsUpIcon = createIcon("ThumbsUpIcon", H_ThumbsUpIcon);
export const TicketIcon = createIcon("TicketIcon", H_Ticket01Icon);
export const TimerIcon = createIcon("TimerIcon", H_Timer01Icon);
export const Trash2 = createIcon("Trash2", H_Delete02Icon);
export const Trash2Icon = createIcon("Trash2Icon", H_Delete02Icon);
export const TrashIcon = createIcon("TrashIcon", H_Delete02Icon);
export const TriangleAlertIcon = createIcon("TriangleAlertIcon", H_Alert02Icon);
export const TrophyIcon = createIcon("TrophyIcon", H_ChampionIcon);
export const TypeIcon = createIcon("TypeIcon", H_TextFontIcon);
export const UnderlineIcon = createIcon("UnderlineIcon", H_TextUnderlineIcon);
export const Undo2Icon = createIcon("Undo2Icon", H_Undo02Icon);
export const UndoIcon = createIcon("UndoIcon", H_UndoIcon);
export const UnlinkIcon = createIcon("UnlinkIcon", H_Unlink01Icon);
export const UnplugIcon = createIcon("UnplugIcon", H_Unlink02Icon);
export const UploadCloudIcon = createIcon("UploadCloudIcon", H_CloudUploadIcon);
export const UploadIcon = createIcon("UploadIcon", H_Upload04Icon);
export const UserIcon = createIcon("UserIcon", H_UserIcon);
export const UserMinusIcon = createIcon("UserMinusIcon", H_UserMinus01Icon);
export const UserPlusIcon = createIcon("UserPlusIcon", H_UserAdd01Icon);
export const UserRoundIcon = createIcon("UserRoundIcon", H_UserIcon);
export const UserXIcon = createIcon("UserXIcon", H_UserRemove01Icon);
export const UsersIcon = createIcon("UsersIcon", H_UserMultipleIcon);
export const UsersRoundIcon = createIcon("UsersRoundIcon", H_UserMultiple02Icon);
export const VideoIcon = createIcon("VideoIcon", H_Video01Icon);
export const WandSparklesIcon = createIcon("WandSparklesIcon", H_MagicWand01Icon);
export const WebhookIcon = createIcon("WebhookIcon", H_WebhookIcon);
export const WrenchIcon = createIcon("WrenchIcon", H_Wrench01Icon);
export const XCircleIcon = createIcon("XCircleIcon", H_CancelCircleIcon);
export const XIcon = createIcon("XIcon", H_Cancel01Icon);
export const ZapIcon = createIcon("ZapIcon", H_ZapIcon);

// No free HugeIcons equivalent yet; keep the lucide glyph for these.
export const HistoryIcon: IconComponent = LucideHistoryIcon;
export const ShieldAlertIcon: IconComponent = LucideShieldAlertIcon;
export const ShieldQuestionIcon: IconComponent = LucideShieldQuestionIcon;
