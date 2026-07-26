<div
	class="stepper {elem_classes.join(' ')}"
	class:hide={visible === false}
	class:hidden={visible === "hidden"}
	id={elem_id}
	style:flex-grow={tab_scale}
	class:compact
>
	{#if has_tabs}
		{#if compact}
			<p class="step-title">
				<strong>Step {($selected_tab_index || 0) + 1}/{tabs.length}:</strong>
				{tabs[$selected_tab_index]?.label || "Walkthrough"}
			</p>
		{/if}
		<div
			class="stepper-wrapper"
			bind:this={stepper_container}
			style:--label-height={label_height + "px"}
		>
			<div
				class="stepper-container"
				bind:this={measurement_container}
				role="tablist"
			>
				{#each tabs as t, i}
					{#if is_visible_tab(t)}
						<div class="step-item">
							<button
								bind:this={step_buttons[i]}
								role="tab"
								class="step-button"
								class:active={t.id === $selected_tab}
								class:completed={t.id < $selected_tab}
								class:pending={t.id > $selected_tab}
								aria-selected={t.id === $selected_tab}
								aria-controls={t.elem_id}
								disabled={!t.interactive || i > $selected_tab_index}
								aria-disabled={!t.interactive || i > $selected_tab_index}
