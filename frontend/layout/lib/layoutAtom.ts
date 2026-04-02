// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import { WOS } from "@/app/store/global";
import { Atom, Getter } from "jotai";

export function getLayoutStateAtomFromTab(tabAtom: Atom<Tab>, get: Getter): WritableGulinObjectAtom<LayoutState> {
    const tabData = get(tabAtom);
    if (!tabData) return;
    const layoutStateOref = WOS.makeORef("layout", tabData.layoutstate);
    const layoutStateAtom = WOS.getGulinObjectAtom<LayoutState>(layoutStateOref);
    return layoutStateAtom;
}
