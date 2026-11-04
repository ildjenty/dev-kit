# UI principles

Screens support users in accomplishing, understanding, or experiencing something. Form hypotheses about who uses them, in what situations, and for what purposes; use those hypotheses to choose content and operations, then verify and revise them through prototypes.

## 1. Start from the user's purpose

- **Applicability**: When deciding what to put on a screen.
- **Exemptions**: When only repairing typos or missing display content, and existing wording definitions, display specifications, and the repair diff confirm that the meaning of displayed content and operations does not change. Waive only reconsideration of users, situations, purposes, and displayed content. Show in the artifact the definitions, specifications, and diff supporting the repair.
- **Principle**: Consider who uses the screen, in what situation, and what they aim to accomplish, understand, or experience; choose content and operations necessary for that purpose. Before drawing, form hypotheses about the purpose and displayed content, and use simple prototypes to check points that cannot be understood without trying layouts or operations. Revise content and purposes according to the results.
- **Violations and signs**:
  - Numbers and text are merely listed, leaving the screen's purpose unclear.
  - Displaying all available data.
  - People opening the screen cannot understand its purpose or how to use it.
  - Adjusting appearance without hypotheses about purpose and displayed content.
  - Not revising the initial proposal even after a prototype reveals mistaken assumptions.
- **Before and after**: "Display every order field in a table" → "A screen for shipping staff to select orders to ship today. Try placing shipping date and status first, then reconsider the necessary content."
- **blocking**: Users, situations, or purposes are undefined.
- **Evidence**: An explanation of users, situations, and purposes, and hypotheses about displayed content. If prototyped, what was checked and revised.

## 2. Match what users already know

- **Applicability**: When deciding layout or operations.
- **Exemptions**: None.
- **Principle**: Decide layout and operations based on existing screens familiar to target users and conventions of the target platform. Use similar products' layouts and operations as references, not as sufficient reasons to adopt them unchanged. When changing familiar layouts or operations, compare benefits for the user's purpose against the burden of relearning.
- **Violations and signs**:
  - Adopting operations different from familiar existing screens or the target platform without a reason.
  - Custom operations require explanation before use.
  - Justifying layout or operations solely by reference to similar products.
- **Before and after**: "Move the save button to match similar products" → "Check the save position on existing screens, then decide placement from the benefits of the change and the burden of relearning."
- **blocking**: None (record as reference information).
- **Evidence**: Existing screens familiar to target users and platform conventions. Differences from referenced similar products and reasons for adopting the layout or operations.

## 3. Choose information based on the burden of the whole task

- **Applicability**: When adding elements or hiding details.
- **Exemptions**: None.
- **Principle**: Choose information to display based on its purpose and the impact of overlooking it, as well as the navigation and memory burden needed for comparison and operations. When hiding details, verify that users can find and compare necessary information. Keep purposeful repeated displays, such as those reducing back-and-forth navigation for verification.
- **Violations and signs**:
  - Elements remain whose removal would not affect achieving the purpose.
  - Hiding information with a large impact if overlooked merely because it is infrequently used.
  - Information needed for comparison is in separate details, requiring navigation and memorization of values.
  - Removing repeated displays increases navigation needed for verification.
  - It is unclear how to open details or where necessary information is located.
- **Before and after**: "Hide contract duration and additional fees in each product's details" → "Keep contract duration and additional fees needed for comparison in the list, and allow opening the remaining details."
- **blocking**: None (record as reference information).
- **Evidence**: Each element's use and the impact of overlooking it. Procedures for opening details, comparison methods, and purposes for keeping repeated displays.

## 4. Make important information visible first

- **Applicability**: When deciding size, weight, color, or position.
- **Exemptions**: When a screen compares equally important candidates and emphasizing a particular candidate first would steer selection before comparison. Limit the exemption to emphasis between candidates; preserve grouping of comparison items and operations.
- **Principle**: Use size, weight, color, and position so that the information most important for a decision is noticed first. Place related elements together and unrelated elements apart. Nearby elements or elements with similar appearances are perceived as a group.
- **Violations and signs**:
  - Everything stands out, so nothing stands out.
  - Unrelated things are placed nearby and appear to be one group.
- **Before and after**: All numbers at the same size → Enlarge the one used for the decision and make the rest smaller
- **blocking**: None.
- **Evidence**: What should be noticed first and how it appears.

## 5. Write in the user's language

- **Applicability**: When deciding labels and wording.
- **Exemptions**: When laws or contracts specify display wording and replacing it with users' language would fail to meet the required display. Limit the exemption to the specified wording.
- **Principle**: Use common terms and users' own words in labels and wording. Status wording must specify whose operation or which processing is being awaited. Check from the reader's perspective whether it could be taken as a demand, rejection, or sarcasm. Choose date and time displays according to whether users need a specific date or elapsed time, and show both when both are needed.
- **Violations and signs**:
  - Labels expose developers' terms or names created during the work.
  - "Preparing" alone can be read as waiting for the user's own work.
  - The same thing has different names on different screens.
  - Failing to handle count-dependent plurals, displaying "1 members" (use `Intl.PluralRules` for plurals).
  - Showing only elapsed time when a specific date is needed, or making users calculate elapsed time from a date when elapsed time is needed.
- **Before and after**: "Waiting for entitlement synchronization" → "Waiting for your plan to take effect (this takes a few minutes)."
- **blocking**: None (record as reference information).
- **Evidence**: A list of wording and what users determine from date and time displays.

## 6. Show state and enable recovery from errors

- **Applicability**: When deciding how to handle operation results and failures.
- **Exemptions**:
  - When preserving input or work position would disclose information to a third party or violate agreed retention or deletion conditions. Waive only preservation of the fields and position information that cannot be retained; warn in advance what will be lost and provide procedures for correction and resumption. Show in the artifact the targets and disclosure paths or retention and deletion conditions.
  - When publicly distinguishing failure causes would disclose information not authorized for disclosure. Waive only publication of that cause information; communicate failure and next steps within what may be disclosed, and retain distinctions among causes internally. Show in the artifact the information to conceal, conditions for sharing public responses, and response examples.
- **Principle**: Show system state, and choose undo, advance display of targets and effects, or additional confirmation according to the likelihood and loss from mistaken operations. For operations involving serious loss, allow users to choose execution after understanding the target and effects. Make the screen understandable without relying on memory, and provide shortcuts for experienced users. On failure, communicate causes and recovery methods, and preserve entered content and work position so users can correct or resume. Provide help wherever use requires explanation.
- **Violations and signs**:
  - After pressing a control, it is unclear what happened.
  - The target or effects of an operation involving serious loss are unclear.
  - Uniformly requiring additional confirmation without considering loss or the likelihood of mistaken operations.
  - On failure, no next steps are given.
  - Failure loses entered content or work position and prevents correction or resumption.
  - No help where use requires explanation.
  - No shortcuts for frequently repeated operations.
- **Before and after**: "A save failure erases input and shows an error screen" → "Preserve input and work position, and explain that saving failed because the connection was lost and that saving can be retried after reconnection."
- **blocking**: Operations involving serious loss can execute without users recognizing the target and effects. Major tasks lose input without warning or cannot be corrected or resumed after failure.
- **Evidence**: Result displays for each operation, targets and effects shown before execution, and methods of preventing mistaken operations. Input and work position before and after failure, recovery guidance, and resumption procedures.

## 7. Design screen states and transitions

- **Applicability**: When deciding possible screen states and transitions between them.
- **Exemptions**: None.
- **Principle**: List states possible on the screen and define displays and operations for states that prevent achieving its purpose. Use empty, loading, failure, partial results, and uneven item counts or text lengths as perspectives for finding omissions. Also design transitions caused by changed conditions, navigation, interruption, or reversed response order, preserving which target and conditions the displayed results belong to. Confirm that displayed values can actually be obtained.
- **Violations and signs**:
  - Only appearance with complete data is defined.
  - Uniformly providing even states impossible on the screen.
  - Long strings overlap or overflow.
  - Sources for displayed values are undefined.
  - JS failure leaves a blank screen, with content unreadable even in a static state.
  - Long names, no matching data, large data volumes, or multiple languages break the display.
  - Long lists scroll slowly.
  - Old responses arriving after conditions change are displayed as results for the new conditions.
  - After navigation or interruption, it becomes unclear which target the processing results belong to.
- **Before and after**: "Provided search success and failure screens" → "Verified that displayed conditions and results remain consistent even when search conditions change and responses arrive in reverse order."
- **blocking**: Reachable states or transitions block major uses. Displayed content is understood as results for a different target or conditions.
- **Evidence**: Reasons for selecting states, and displays and operations for each. Display results when operation and response orders vary.

## 8. Ensure accessibility

- **Applicability**: When deciding colors, operations, screen-reader output, or motion.
- **Exemptions**: In tests measuring perception itself, when presenting identifying stimulus content through another perceptual channel would invalidate measurement, waive only that alternative stimulus presentation. Show in the artifact the test specification, target stimuli, and why alternative presentation invalidates measurement. Apply this principle to explanations, starting, interruption, answers, and checking results.
- **Principle**: Choose colors that keep text distinct from the background and content readable. Make necessary information and operation meanings understandable through visual display, keyboard operation, and screen readers, allowing completion of major tasks. Represent kinds and states so they can be distinguished without misunderstanding; do not rely on color alone. Add wording needed to understand icons or positions whose meanings are not shared. Limit motion to what helps understand state changes, and prevent it from delaying repeated operations or disrupting attention or physical well-being. Convey necessary changes even when users choose reduced motion.
- **Violations and signs**:
  - Some operations cannot be reached by keyboard.
  - Screen-reader output does not communicate options, selection state, or operation meanings or results.
  - Pale text blends into the background.
  - State is shown by color alone, without shape or wording.
  - Custom icons or positions are not understood, causing confusion about kinds or states.
  - Unclickable elements look like buttons.
  - Touch devices have hover-dependent operations unusable without hover.
  - Motion delays repeated operations or disrupts attention or physical well-being.
  - Reduced-motion settings obscure necessary state changes.
- **Before and after**: "Check only opening and closing with the keyboard" → "Verify that keyboard and screen-reader users can understand options and selection state and complete selection through confirmation."
- **blocking**: Keyboard or screen-reader use impedes understanding or operations so that major tasks cannot be completed. Kinds or states are distinguished by color alone.
- **Evidence**: Readability from color choices and results of understanding information and completing major tasks by keyboard and screen reader. How kinds and states are distinguished, and operation checks with normal and reduced motion.

## 9. Check rendered results and interactions

- **Applicability**: Before saying a screen is complete.
- **Exemptions**: When horizontal scrolling within an element is necessary to preserve spatial relationships in wide tables or diagrams, and fitting them to width would omit information needed for comparison or obscure correspondence. Permit horizontal scrolling within the element. Limit the exemption to that scrolling; still check rendered results and interactions.
- **Principle**: Do not judge appearance from numbers or explanations alone; inspect rendered results as images. Verify properties dependent on time and interaction, such as responsiveness, waiting, focus movement, and animation, through actual interaction. Limit evaluation to what the evidence confirms, and mark unverified areas or properties `NOT ASSESSED`.
- **Violations and signs**:
  - Calling a screen "complete" without ever seeing its rendered result.
  - Treating responsiveness, waiting, focus movement, or animation as verified from static images alone.
  - Users notice overlapping or overflowing text first.
  - `100vh` cuts off the bottom of the screen.
  - Unintended horizontal scrolling across the whole page.
  - A dark screen flashes white during loading.
  - Insufficient contrast makes text over images or videos unreadable.
  - Fonts fail to load, causing unintended fallback fonts to appear.
  - Placeholder text (lorem, `TODO`) remains instead of actual wording.
  - Using purple gradients, glowing spheres, or blurred blobs as default decoration instead of choosing colors and shapes from content.
  - Digits shift position whenever numbers change.
  - Large headings look too spread out, or body text is hard to read.
  - Buttons feel slow to respond.
  - Avatars are distorted into ovals.
  - Dropdowns feel sluggish.
  - Focus moves somewhere unintended after an operation, preventing continued work.
  - Popovers appear from unintended positions.
  - Animations stutter or unintended elements move.
- **Before and after**: "Saw the image, so dropdown responsiveness is fine too" → "Checked appearance in an image, then actually opened and closed the dropdown to verify responsiveness and focus movement. Marked unverified properties `NOT ASSESSED`."
- **blocking**: Rendered results or actual interaction reveal impediments that prevent reading necessary content, operating the screen, or continuing work.
- **Evidence**: Images used to check appearance, actual interaction procedures, and observations of responsiveness, waiting, focus movement, and animation.

## 10. Preserve the meaning of numbers and comparisons

- **Applicability**: When displaying numbers, aggregates, graphs, or comparison results on a screen.
- **Exemptions**: In tasks measuring the ability to detect missing numerical assumptions or flawed comparison conditions, when making those assumptions or flaws explicit would reveal the answer. Waive only the requirement to supplement evaluated assumptions and indicate differing conditions, missing measurements, or provisional results in the task materials; identify them as task materials and prevent their use as a basis for actual decisions or transactions. Apply this principle to explanations and evaluation-result displays. Show in the artifact the ability being assessed, target materials, information whose addition would invalidate evaluation, and how it will be presented in explanations.
- **Principle**: Show subjects, units, periods, denominators, and aggregation scope needed to interpret numbers. When handling values with different comparison conditions, reconsider the comparison method or display the differences clearly. Indicate missing measurements or provisional results as such; do not treat them as final values for the whole.
- **Violations and signs**:
  - Comparing values for different periods or denominators as though conditions were identical.
  - Displaying filtered totals as overall totals.
  - Displaying results only from the retrievable scope as results for all items.
  - Missing measurements or provisional results appear to be final overall values.
- **Before and after**: "Compare the current partial month with the entire previous month and display the month-over-month change" → "Compare values for the same number of elapsed days in both months, and display the compared periods too."
- **blocking**: Missing assumptions needed for interpretation or unsuitable comparison methods cause users to misunderstand subjects, trends, or certainty.
- **Evidence**: Sources of displayed values, aggregation and comparison conditions, and display examples showing that users can understand them.
