# Writing principles

The purpose is text that readers can understand on the first reading.

## 1. Put conclusions and information needed for human decisions first

- **Applicability**: When presenting documents, reports, or PR descriptions, or recording or reporting check results.
- **Exemptions**:
  - When reordering original records that preserve event order would damage sequence or evidentiary value. Waive only ordering requirements for the portions that must be preserved; keep their order, information needed for decisions, and complete check records. Show in the artifact the portions requiring preservation and why; if a summary can be added outside the record, put conclusions and information needed for decisions first.
  - When the recipient requires a specified order for acceptance and permits neither conclusions first nor added summaries. Waive only ordering requirements for the specified portions; follow the required order and retain information needed for decisions and complete check records. Show in the artifact a reference to the required format and its conditions.
- **Principle**: Put conclusions and information readers need first, according to the document's purpose. Reports and PR descriptions must state the scope of changes, verification results, and decisions, unverified matters, and risks affecting acceptance or safe use. When requesting a response, state the action explicitly, and also present reports that require no response. Preserve all findings in check outputs and records. Reports for people must show decisions currently needed and relevant results, with access to complete records and omitted details. Include implementation details needed to understand changes, verification results, or decisions. In Scope, state what is and is not included; in Tradeoffs, state rejected options whose acceptance readers need to assess. Do not name decision-makers unnecessary for the decision.
- **Violations and signs**:
  - Starting with background and placing conclusions or information needed for decisions at the end.
  - Change scope or verification results cannot be understood from reports or PR descriptions.
  - Omitting decisions, unverified matters, or risks affecting change acceptance or safe use.
  - Necessary information is missed amid implementation details or lists of findings unrelated to the decision.
  - Repeating in body text what headings or bullets already convey.
  - When action is needed, the requested action is unclear, or a report requiring no response asks for action.
  - Check records lack findings, or complete records and omitted details cannot be reached from the report.
- **Before and after**: "The changed files are listed below. The provisional judgment is at the end" → "Changed feature controls by plan. Verification of the target plan is complete, but behavior under existing contracts remains unverified. Please decide acceptance with that unverified scope in mind."
- **blocking**: Information affecting a decision is missing or placed where readers miss it, leading to mistaken decisions.
- **Evidence**: Readers can find conclusions and necessary information, judge change scope, verification results, and whether a response is needed, and access complete records and omitted details.

## 2. Define assumptions about readers and write independently of the conversation

- **Applicability**: When choosing names or terms for readers, or turning conversation or source material into documents, reports, PR descriptions, or review comments.
- **Exemptions**: When original statements or materials must be quoted for comparison, and standardizing terms or rephrasing first-person wording would prevent checking differences from the original or the speaker's intent. Preserve original text in quotations and give sources and assumptions needed for understanding outside the quote.
- **Principle**: Define intended readers and the knowledge and reference materials that may be assumed. For function names, names created during work, or number-only references whose meaning those assumptions do not convey, add purpose explanations or references, or rephrase meaningfully. Use words from documents and code, and use the same terms for the same things. Preserve decision assumptions shared only in conversation in the text or provide references. Check whether readers who did not observe the work can make the relevant decisions or perform the work using necessary materials, and apply the same approach to similar cases not discussed in the conversation. Follow principles 1 and 2 of `principles/verification.md` for verifying facts and inferences stated in writing. When supplementing explanations of requests or materials, distinguish source contents from additions and give evidence. In PR descriptions and review comments, avoid first person and make the changes or decisions themselves the subject.
- **Violations and signs**:
  - Omitting explanations or continuing to explain known content without defining readers' knowledge or reference materials.
  - Explaining through numbers or names alone, leaving intended readers unable to infer the purpose.
  - Names, abbreviations, or metaphors created during conversation, or assumptions incomprehensible without it.
  - Calling the same thing both "placement" and "location." Different terms suggest different things.
  - Forcing terms normally used in katakana in Japanese (table, layer, retry, format) into native Japanese equivalents.
  - Applicability is limited to examples from the conversation, leaving judgments for similar cases unclear.
  - Writing supplementary explanations as though they were source contents.
  - Using first person in PR descriptions or review comments.
- **Before and after**: "Add entitlements to #48" → "Add a mechanism controlling available features by plan (#48)."
- **blocking**: Names or inconsistent terms cause readers to misunderstand meanings. For fact verification, apply principle 1 of `principles/verification.md` to written claims too, including unverified content presented as verified.
- **Evidence**: Intended readers can use the assumed materials to understand necessary distinctions and make relevant decisions or perform work without knowing the conversation. Correspondence between source contents and additions, or check results from the `wording` perspective.

## 3. Use concrete language that makes clear who does what

- **Applicability**: When describing actions, states, or judgment criteria.
- **Exemptions**:
  - When an existing misunderstanding needs correction, and avoiding "not X but Y" contrasts would obscure the difference between the mistaken and correct understanding. Waive only the ban on "not X but Y" contrasts. Show in the artifact the misunderstanding being corrected and the correct understanding.
  - When multiple conditions or operations and their results are matched in the same item order for reference. Waive only the ban on repeated sentence structures; preserve concrete subjects, actions, and judgment criteria. Show in the artifact how each sentence's condition or operation corresponds to its result.
- **Principle**: Use concrete subjects and verbs to state who or what acts, what they do, and what happens. Do not obscure meaning through metaphors, personification, literal translations from English, or nominalization; state judgment criteria concretely. Do not use "not X but Y" contrasts or repeat the same sentence structure.
- **Violations and signs**:
  - Metaphors or personification such as "what sits on top," "calls itself," or "weakens the defenses." Write concretely: "later steps and code using this," "state explicitly in the output," or "allows reading another store's data."
  - Sentences make objects act like people, or use literal English-to-Japanese forms equivalent to "can do," "have," or "by doing." In Japanese, replace the verbose form of "can verify" with its concise potential form.
  - Nominalizations such as "handling duplicate registration" obscure who does what. Write "Fix it so the same store cannot be registered twice."
  - Wording such as "appropriately," "as needed," or "whichever is easier to understand" permits different judgments by different people.
  - Relying on expressions that merely combine abstract nouns and verbs.
  - Contrasts that do not correct misunderstandings, or repetition of the same contrast structure.
- **Before and after**: "Weaken the defenses" → "The app becomes able to read another store's data."
- **blocking**: Metaphors, literal translations, nominalizations, or wording allowing differing judgments make the meaning incomprehensible.
- **Evidence**: Check results from the `wording` perspective.

## 4. Use the length and structure needed for understanding, execution, and reference

- **Applicability**: When deciding explanation length or structure, or choosing prose, bullets, or tables.
- **Exemptions**: Tables may be used for purposes other than comparison when the recipient requires a table for acceptance, or when converting a reference table that looks up the same kinds of items by identifier into prose would make correspondence hard to follow. Show a reference to the required format, or the lookup identifiers, corresponding items, and how to search. Waive only restrictions on table use; preserve the information and explanation needed for understanding, execution, and reference.
- **Principle**: Choose information volume and structure that let readers accomplish understanding, execution, or reference. Add concrete examples where abstract explanations alone would cause mistaken distinctions or application. Use tables for comparison, not for listing explanations.
- **Violations and signs**:
  - Prioritizing brevity compresses explanations and omits content needed for understanding or execution.
  - All information exists, but readers cannot find the relevant reference or action.
  - No concrete examples where abstract explanations alone lead to mistaken distinctions or application.
  - Using tables to list explanations.
- **Before and after**: "Omit concrete examples to shorten the explanation" → "Add examples where application is easily mistaken, and structure the text so readers can choose operations."
- **blocking**: None.
- **Evidence**: Readers can find information needed for their purpose and understand distinctions, application, and operations.

## 5. Define authoritative sources and update ownership to prevent conflicting or contradictory documents

- **Applicability**: When creating or updating documents or republishing summaries or examples.
- **Exemptions**: When usage purposes, update timing, or length differ and combining documents would make necessary information harder to find, waive only the requirement to place explanations close together and allow separate documents. State why they are separate and each document's use. Define authoritative sources, update ownership, correspondence to republished content, and update methods; make constraints needed for the same work accessible from where it is performed.
- **Principle**: Define the authoritative source and assign update responsibility to one owner. Republish summaries, examples, or derived views needed for understanding or execution in a form that makes their correspondence to the authoritative source and update method clear. Identify authoritative code or configuration in explanations of code or configuration too. Do not create competing, independently updated explanations for the same subject, version, and purpose. Keep explanations, constraints, and procedures needed for the same work or decision close together. Match headings to content and diagrams to body text.
- **Violations and signs**:
  - Stated counts do not match listed counts.
  - Headings do not match content, or diagrams do not match body text.
  - Authoritative sources, corresponding portions, or update methods for summaries or examples are unclear.
  - Explanations for the same subject, version, and purpose are updated through separate decisions.
  - Contracts for different versions or records at the time of change lack applicability information or access to the current version.
  - Constraints and procedures needed for the same work are far apart, so procedures alone allow constraints to be missed.
- **Before and after**: "Update the same configuration example separately" → "Define the configuration example's authoritative source and update owner; republish necessary examples in procedures with references to the corresponding source portions and update methods."
- **blocking**: None (detected by the `consistency` perspective).
- **Evidence**: Check results from the `consistency` perspective and correspondence among authoritative sources, update ownership, and republished content. Each document's applicability is clear, and information needed for the same work is accessible.

## 6. Distinguish current content from history and plans

- **Applicability**: When explaining current specifications, retaining investigation reports or decision records, or recording work whose execution or decisions depend on schedules.
- **Exemptions**: When primary materials omit the year and it cannot be identified within the verified scope, waive only the requirement to state the year, preserving the original without supplying one. Show the source and verified scope, and explicitly state that the year is unverified. Do not assert chronology that requires the year to be established.
- **Principle**: In current-specification documents, distinguish current content from past content so past descriptions are not read as current specifications. In investigation reports and decision records, retain the history supporting decisions and the time of observation. For work whose execution or decisions depend on schedules, state necessary plans and decision deadlines. Distinguish fixed constraints, agreed goals, and duration estimates, and update them when they change. Do not bring irrelevant intermediate history into explanations of current specifications. Write dates starting with the year.
- **Violations and signs**:
  - Past content and current content are indistinguishable.
  - Investigation reports or decision records lack history or observation times supporting decisions.
  - Plans or decision deadlines needed for execution or decisions are missing.
  - Fixed constraints, agreed goals, and estimates are mixed.
  - Changed schedules are not updated, or dates lack years.
- **Before and after**: "List initial and revised specifications without distinction" → "Identify current specifications, and retain the history of changes with their times in decision records."
- **blocking**: None.
- **Evidence**: Readers can distinguish the time described and current validity, and check history and observation times supporting decisions, as well as schedules and decision deadlines needed for work.

## 7. When asking for decisions, provide concise options with a recommendation and reasons

- **Applicability**: When asking people to decide.
- **Exemptions**:
  - When independent evaluations of options are needed and presenting a recommendation first would bias responses and hide concerns or alternatives. Waive only presentation of a recommendation. Show in the artifact differences between options and information needed for decisions.
  - When defining options or preferences first requires confirming constraints decided by a person or facts only that person knows. Waive only presentation of options and recommendations in that confirmation; do not apply this exemption to matters decidable from previous answers. Show in the artifact the missing assumptions and candidates that change according to the answer.
- **Principle**: Check that differences between options and reasons for recommendations help human decisions. Limit the number of matters asked about at once. Do not ask what can be decided from previous answers.
- **Violations and signs**:
  - Asking again about matters decidable from previous answers.
  - Asking about many matters at once.
- **Before and after**: "What should we do?" → "Revise the existing document or create a separate one? Revising the existing document is recommended to keep one place to update."
- **blocking**: None.
- **Evidence**: Stated options and recommendations.

## 8. Define a diagram's purpose and show relationship kinds and directions

- **Applicability**: When explaining structure or behavior through diagrams.
- **Exemptions**: When notation or a legend makes relationship meanings clear, omit repeated identical labels on each arrow. Duplicate labels overlapping arrows or other content make relationships hard to follow.
- **Principle**: Define what the diagram shows and make relationship kinds and directions understandable. For behavior diagrams, include actions and conditions needed to understand them. Do not add unverified behavior to structure diagrams.
- **Violations and signs**:
  - Boxes or arrows have unclear meanings, making dependencies indistinguishable from runtime behavior.
  - Relationship directions or conditions enabling behavior cannot be understood.
  - Adding unverified behavior such as reading or writing based only on dependency records.
- **Before and after**: "Label an arrow 'writes' when only the dependency was verified" → "State that the diagram shows dependencies, and show direction toward dependencies in the legend."
- **blocking**: None.
- **Evidence**: The diagram corresponds to its supporting records, and readers can distinguish relationship kinds, directions, and conditions needed for behavior.

## 9. Distinguish information you can read from information you may share

- **Applicability**: When transferring content from conversations, logs, real data, or private materials into reports, PRs, or documents.
- **Exemptions**: When real data authorized for disclosure must be used, and substitution would lose failure reproduction conditions or evidence needed to compare records. Limit the scope without substitution to information necessary for the purpose.
- **Principle**: Check the target readers and disclosure scope, and communicate information authorized for disclosure only to the extent necessary for the purpose. Substitute real data while preserving structures and conditions needed for decisions. Do not reproduce original private information if permission to disclose it cannot be confirmed.
- **Violations and signs**:
  - Reproducing original text solely because it could be read.
  - Personally identifying information or credentials unnecessary for the explanation remain.
  - Substitution also changes conditions causing failure.
- **Before and after**: "Use customers' real names and order text unchanged in a reproduction example" → "Replace real names with fictional ones while preserving the character count and input conditions that caused the problem."
- **blocking**: Includes information not authorized for disclosure to readers, or private information whose disclosure permission cannot be confirmed.
- **Evidence**: Handling of the source, target readers and disclosure scope, necessity of retained information, and preservation of conditions needed for explanation or verification after substitution.
