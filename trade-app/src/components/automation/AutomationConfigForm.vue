<template>
  <div class="automation-config-form">
    <!-- Header -->
    <div class="form-header">
      <div class="header-content">
        <h1 class="form-title">
          <i class="pi pi-cog"></i>
          {{ isEditMode ? 'Edit' : 'Create' }} Automation Config
        </h1>
        <p class="form-subtitle">
          Configure automated credit spread trading criteria
        </p>
      </div>
      <div class="header-actions">
        <Button
          icon="pi pi-times"
          label="Cancel"
          class="p-button-text"
          @click="cancel"
        />
        <Button
          icon="pi pi-check"
          label="Save Config"
          class="p-button-primary"
          @click="saveConfig"
          :loading="isSaving"
        />
      </div>
    </div>

    <!-- Loading State -->
    <div v-if="isLoading" class="loading-state">
      <div class="loading-spinner"></div>
      <p>Loading config...</p>
    </div>

    <!-- Form Content -->
    <div v-else class="form-content">
      <!-- Basic Info Section -->
      <div class="form-section">
        <h2 class="section-title">Basic Information</h2>
        <div class="form-grid">
          <div class="form-field">
            <label for="name">Config Name</label>
            <InputText
              id="name"
              v-model="config.name"
              placeholder="e.g., NDX Morning Put Spread"
              :class="{ 'p-invalid': errors.name }"
            />
            <small v-if="errors.name" class="p-error">{{ errors.name }}</small>
          </div>
          <div class="form-field">
            <label for="symbol">Symbol</label>
            <Dropdown
              id="symbol"
              v-model="config.symbol"
              :options="symbols"
              placeholder="Select Symbol"
              :class="{ 'p-invalid': errors.symbol }"
            />
            <small v-if="errors.symbol" class="p-error">{{ errors.symbol }}</small>
          </div>
        </div>
      </div>

      <!-- Entry Time Section -->
      <div class="form-section">
        <h2 class="section-title">Entry Time & Schedule</h2>
        <div class="form-grid">
          <div class="form-field">
            <label for="entryTime">Entry Time</label>
            <InputText
              id="entryTime"
              v-model="config.entry_time"
              placeholder="HH:MM (e.g., 12:25)"
              :class="{ 'p-invalid': errors.entry_time }"
            />
            <small class="field-hint">24-hour format (e.g., 12:25 for 12:25 PM)</small>
            <small v-if="errors.entry_time" class="p-error">{{ errors.entry_time }}</small>
          </div>
          <div class="form-field">
            <label for="timezone">Timezone</label>
            <Dropdown
              id="timezone"
              v-model="config.entry_timezone"
              :options="timezones"
              placeholder="Select Timezone"
            />
          </div>
          <div class="form-field">
            <label for="recurrence">Recurrence</label>
            <Dropdown
              id="recurrence"
              v-model="config.recurrence"
              :options="recurrenceOptions"
              optionLabel="label"
              optionValue="value"
              placeholder="Select Recurrence"
            />
            <small class="field-hint">{{ getRecurrenceHint() }}</small>
          </div>
        </div>
      </div>

      <!-- Indicators Section -->
      <div class="form-section">
        <h2 class="section-title">Entry Indicators</h2>
        <p class="section-description">
          All enabled indicators must pass for a trade to be executed. Add indicators using the button below.
        </p>
        
        <!-- Empty state when no indicators -->
        <div v-if="!config.indicators.length" class="indicators-empty">
          <i class="pi pi-filter"></i>
          <p>No indicators configured. Add indicators to define entry criteria.</p>
        </div>
        
        <div v-else class="indicators-list" :class="{ 'mobile-indicators': isMobile }">
          <div
            v-for="indicator in config.indicators"
            :key="indicator.id"
            class="indicator-row"
            :class="{ 'disabled': !indicator.enabled }"
          >
            <div class="indicator-toggle">
              <InputSwitch v-model="indicator.enabled" />
            </div>
            <div class="indicator-type">
              <span class="type-label">{{ formatIndicatorTypeWithParams(indicator) }}</span>
              <span v-if="!isMobile" class="type-description">{{ getIndicatorDescription(indicator.type) }}</span>
            </div>
            <div class="indicator-config" :class="{ 'dimmed': !indicator.enabled }">
              <Dropdown
                v-model="indicator.operator"
                :options="displayOperators"
                optionLabel="label"
                optionValue="value"
                class="operator-dropdown"
                :class="{ 'mobile-operator': isMobile }"
                :disabled="!indicator.enabled"
              />
              <InputNumber
                v-model="indicator.threshold"
                :minFractionDigits="indicator.type === 'calendar' ? 0 : 2"
                :maxFractionDigits="indicator.type === 'calendar' ? 0 : 2"
                class="threshold-input"
                :class="{ 'mobile-threshold': isMobile }"
                :disabled="!indicator.enabled"
                :placeholder="indicator.type === 'calendar' ? '0 or 1' : 'Value'"
              />
              <template v-if="getIndicatorParams(indicator.type).length > 0 && indicator.params">
                <div
                  v-for="paramDef in getIndicatorParams(indicator.type)"
                  :key="paramDef.key"
                  class="param-input-group"
                >
                  <label class="param-label">{{ paramDef.label }}</label>
                  <InputNumber
                    v-model="indicator.params[paramDef.key]"
                    :min="paramDef.min"
                    :max="paramDef.max"
                    :step="paramDef.step"
                    :minFractionDigits="paramDef.type === 'float' ? 1 : 0"
                    :maxFractionDigits="paramDef.type === 'float' ? 2 : 0"
                    class="param-value-input"
                    :disabled="!indicator.enabled"
                  />
                </div>
              </template>
              <InputText
                v-if="getIndicatorNeedsSymbol(indicator.type)"
                v-model="indicator.symbol"
                :placeholder="getIndicatorDefaultSymbol(indicator.type)"
                class="symbol-input"
                :class="{ 'mobile-symbol': isMobile }"
                :disabled="!indicator.enabled"
              />
              <!-- Mobile test result - shown inline after Test All -->
              <span 
                v-if="isMobile && indicatorResults[indicator.id]" 
                class="mobile-test-result" 
                :class="getIndicatorResultClass(indicatorResults[indicator.id])"
                :title="indicatorResults[indicator.id].stale ? `Stale: ${indicatorResults[indicator.id].error}` : ''"
              >
                <span v-if="indicatorResults[indicator.id].stale" class="stale-icon">⚠</span>
                {{ indicatorResults[indicator.id].value?.toFixed(2) || 'N/A' }}
              </span>
            </div>
            <div v-if="!isMobile" class="indicator-test">
              <Button
                icon="pi pi-sync"
                class="p-button-text p-button-sm"
                title="Test this indicator"
                @click="testIndicator(indicator)"
                :loading="testingIndicator === indicator.id"
                :disabled="!indicator.enabled"
              />
              <span 
                v-if="indicatorResults[indicator.id]" 
                class="test-result" 
                :class="getIndicatorResultClass(indicatorResults[indicator.id])"
                :title="indicatorResults[indicator.id].stale ? `Stale: ${indicatorResults[indicator.id].error}` : ''"
              >
                <span v-if="indicatorResults[indicator.id].stale" class="stale-icon">⚠</span>
                {{ indicatorResults[indicator.id].value?.toFixed(2) || 'N/A' }}
              </span>
            </div>
            <div class="indicator-remove">
              <Button
                icon="pi pi-times"
                class="p-button-text p-button-danger p-button-sm"
                title="Remove indicator"
                @click="removeIndicator(indicator.id)"
              />
            </div>
          </div>
        </div>

        <div class="indicators-actions">
          <Button
            icon="pi pi-plus"
            label="Add Indicator"
            class="p-button-outlined"
            @click="showAddIndicatorDialog = true"
          />
          <Button
            v-if="config.indicators.length > 0"
            icon="pi pi-play"
            label="Test All"
            class="p-button-outlined"
            @click="testAllIndicators"
            :loading="testingAll"
          />
          <span v-if="allIndicatorsResult !== null" class="all-result" :class="allIndicatorsResult ? 'passed' : 'failed'">
            {{ allIndicatorsResult ? 'All Passed' : 'Some Failed' }}
          </span>
        </div>
      </div>

      <!-- Add Indicator Dialog -->
      <Dialog
        v-model:visible="showAddIndicatorDialog"
        header="Add Indicator"
        :modal="true"
        :style="{ width: '400px' }"
      >
        <div class="add-indicator-content">
          <p class="add-indicator-description">
            Select an indicator type to add. You can add multiple instances of the same type.
          </p>
          <div class="indicator-type-options">
            <div v-for="(types, category) in groupedIndicatorTypes" :key="category">
              <h4 class="category-header">{{ categoryLabels[category] || category }}</h4>
              <div
                v-for="type in types"
                :key="type.value"
                class="indicator-type-option"
                @click="addIndicator(type.value)"
              >
                <div class="option-header">
                  <span class="option-label">{{ type.label }}</span>
                </div>
                <p class="option-description">{{ type.description }}</p>
              </div>
            </div>
          </div>
        </div>
      </Dialog>

      <!-- Trade Configuration Section -->
      <div class="form-section">
        <h2 class="section-title">Trade Configuration</h2>
        
        <div class="form-grid">
          <div class="form-field">
            <label for="strategy">Strategy</label>
            <Dropdown
              id="strategy"
              v-model="config.trade_config.strategy"
              :options="strategies"
              optionLabel="label"
              optionValue="value"
              placeholder="Select Strategy"
            />
          </div>

          <!-- Single-side config: Target Delta + Width for credit spreads -->
          <template v-if="!isIronCondor">
            <div class="form-field">
              <label for="targetDelta">Target Delta</label>
              <InputNumber
                id="targetDelta"
                v-model="config.trade_config.target_delta"
                :minFractionDigits="2"
                :maxFractionDigits="3"
                :min="0.01"
                :max="0.50"
                placeholder="e.g., 0.05"
              />
              <small class="field-hint">Short strike delta (e.g., 0.05 = 5 delta)</small>
            </div>
            <div class="form-field">
              <label for="width">Spread Width</label>
              <InputNumber
                id="width"
                v-model="config.trade_config.width"
                :min="5"
                :max="100"
                placeholder="e.g., 20"
              />
              <small class="field-hint">Strike width in points</small>
            </div>
          </template>

          <div class="form-field">
            <label for="maxCapital">Max Capital</label>
            <InputNumber
              id="maxCapital"
              v-model="config.trade_config.max_capital"
              mode="currency"
              currency="USD"
              :min="100"
              placeholder="e.g., 5000"
            />
            <small class="field-hint">Maximum capital to risk on this trade</small>
          </div>
        </div>

        <!-- Iron Condor per-side configuration -->
        <template v-if="isIronCondor">
          <h3 class="subsection-title">Put Side (Bull Put Spread)</h3>
          <div class="form-grid">
            <div class="form-field">
              <label for="putTargetDelta">Target Delta</label>
              <InputNumber
                id="putTargetDelta"
                v-model="config.trade_config.put_side_config.target_delta"
                :minFractionDigits="2"
                :maxFractionDigits="3"
                :min="0.01"
                :max="0.50"
                placeholder="e.g., 0.05"
              />
              <small class="field-hint">Put short strike delta</small>
            </div>
            <div class="form-field">
              <label for="putWidth">Spread Width</label>
              <InputNumber
                id="putWidth"
                v-model="config.trade_config.put_side_config.width"
                :min="5"
                :max="100"
                placeholder="e.g., 50"
              />
              <small class="field-hint">Put side strike width in points</small>
            </div>
          </div>

          <h3 class="subsection-title">Call Side (Bear Call Spread)</h3>
          <div class="form-grid">
            <div class="form-field">
              <label for="callTargetDelta">Target Delta</label>
              <InputNumber
                id="callTargetDelta"
                v-model="config.trade_config.call_side_config.target_delta"
                :minFractionDigits="2"
                :maxFractionDigits="3"
                :min="0.01"
                :max="0.50"
                placeholder="e.g., 0.07"
              />
              <small class="field-hint">Call short strike delta</small>
            </div>
            <div class="form-field">
              <label for="callWidth">Spread Width</label>
              <InputNumber
                id="callWidth"
                v-model="config.trade_config.call_side_config.width"
                :min="5"
                :max="100"
                placeholder="e.g., 30"
              />
              <small class="field-hint">Call side strike width in points</small>
            </div>
          </div>
        </template>

        <h3 class="subsection-title">Order Settings</h3>
        <div class="form-grid">
          <div class="form-field">
            <label for="orderType">Order Type</label>
            <Dropdown
              id="orderType"
              v-model="config.trade_config.order_type"
              :options="orderTypes"
              optionLabel="label"
              optionValue="value"
            />
          </div>
          <div class="form-field">
            <label for="timeInForce">Time in Force</label>
            <Dropdown
              id="timeInForce"
              v-model="config.trade_config.time_in_force"
              :options="timeInForceOptions"
              optionLabel="label"
              optionValue="value"
            />
          </div>
        </div>

        <h3 class="subsection-title">Price Ladder Settings</h3>
        <div class="form-grid">
          <div class="form-field">
            <label for="startingOffset">Starting Price Offset</label>
            <InputNumber
              id="startingOffset"
              v-model="config.trade_config.starting_offset"
              :minFractionDigits="2"
              :maxFractionDigits="2"
              :min="0"
              :max="2.00"
              placeholder="e.g., 0.10"
            />
            <small class="field-hint">Amount below mid to start (e.g., 0.10 for more aggressive fill)</small>
          </div>
          <div class="form-field">
            <label for="minCredit">Minimum Credit</label>
            <InputNumber
              id="minCredit"
              v-model="config.trade_config.min_credit"
              :minFractionDigits="2"
              :maxFractionDigits="2"
              :min="0"
              :max="10.00"
              placeholder="e.g., 0.30"
            />
            <small class="field-hint">Stop if credit falls below this threshold</small>
          </div>
          <div class="form-field">
            <label for="priceLadderStep">Price Ladder Step</label>
            <InputNumber
              id="priceLadderStep"
              v-model="config.trade_config.price_ladder_step"
              :minFractionDigits="2"
              :maxFractionDigits="2"
              :min="0.01"
              :max="1.00"
              placeholder="e.g., 0.05"
            />
            <small class="field-hint">Increment for price improvement attempts</small>
          </div>
          <div class="form-field">
            <label for="maxAttempts">Max Attempts</label>
            <InputNumber
              id="maxAttempts"
              v-model="config.trade_config.max_attempts"
              :min="1"
              :max="50"
              placeholder="e.g., 10"
            />
            <small class="field-hint">Maximum price improvement attempts</small>
          </div>
          <div class="form-field">
            <label for="attemptInterval">Attempt Interval (sec)</label>
            <InputNumber
              id="attemptInterval"
              v-model="config.trade_config.attempt_interval"
              :min="5"
              :max="300"
              placeholder="e.g., 30"
            />
            <small class="field-hint">Seconds between attempts</small>
          </div>
          <div class="form-field">
            <label for="deltaDriftLimit">Delta Drift Limit</label>
            <InputNumber
              id="deltaDriftLimit"
              v-model="config.trade_config.delta_drift_limit"
              :minFractionDigits="2"
              :maxFractionDigits="3"
              :min="0.001"
              :max="0.10"
              placeholder="e.g., 0.01"
            />
            <small class="field-hint">Max delta change before re-selecting strikes</small>
          </div>
        </div>

        <h3 class="subsection-title">Expiration Settings</h3>
        <div class="form-grid">
          <div class="form-field">
            <label for="expirationMode">Expiration Mode</label>
            <Dropdown
              id="expirationMode"
              v-model="config.trade_config.expiration_mode"
              :options="expirationModes"
              optionLabel="label"
              optionValue="value"
              placeholder="Select Expiration"
            />
            <small class="field-hint">Select which expiration to trade</small>
          </div>
          <div class="form-field" v-if="config.trade_config.expiration_mode === 'custom'">
            <label for="customExpiration">Custom Expiration Date</label>
            <InputText
              id="customExpiration"
              v-model="config.trade_config.custom_expiration"
              placeholder="YYYY-MM-DD"
            />
            <small class="field-hint">Specific date in YYYY-MM-DD format</small>
          </div>
        </div>

        <!-- Strike Preview Section -->
        <h3 class="subsection-title">Strike Preview</h3>
        <div class="strike-preview-section">
          <Button
            icon="pi pi-search"
            label="Preview Strikes"
            class="p-button-outlined"
            @click="loadStrikePreview"
            :loading="loadingPreview"
            :disabled="!config.symbol || !config.trade_config.strategy"
          />
          
          <div v-if="strikePreview" class="strike-preview-content" :class="{ 'mobile-preview': isMobile }">
            <div class="preview-expiry">
              <strong>Exp:</strong> {{ strikePreview.spread?.expiry || 'N/A' }}
            </div>

            <!-- Iron Condor: show both sides -->
            <template v-if="isIronCondor">
              <h4 class="ic-side-title">Put Side (Bull Put Spread)</h4>
              <div class="preview-legs">
                <div class="preview-leg short-leg">
                  <h4>Short Put (Sell)</h4>
                  <div class="leg-details">
                    <div class="leg-row">
                      <span class="leg-label">Strike:</span>
                      <span class="leg-value">{{ strikePreview.put_side?.short_leg?.strike?.toFixed(0) || 'N/A' }}</span>
                    </div>
                    <div class="leg-row">
                      <span class="leg-label">Delta:</span>
                      <span class="leg-value">{{ strikePreview.put_side?.short_leg?.delta?.toFixed(3) || 'N/A' }}</span>
                    </div>
                    <div class="leg-row">
                      <span class="leg-label">Bid/Ask:</span>
                      <span class="leg-value">{{ formatPrice(strikePreview.put_side?.short_leg?.bid) }}/{{ formatPrice(strikePreview.put_side?.short_leg?.ask) }}</span>
                    </div>
                    <div class="leg-row">
                      <span class="leg-label">Mid:</span>
                      <span class="leg-value highlight">{{ formatPrice(strikePreview.put_side?.short_leg?.mid) }}</span>
                    </div>
                  </div>
                </div>
                <div class="preview-leg long-leg">
                  <h4>Long Put (Buy)</h4>
                  <div class="leg-details">
                    <div class="leg-row">
                      <span class="leg-label">Strike:</span>
                      <span class="leg-value">{{ strikePreview.put_side?.long_leg?.strike?.toFixed(0) || 'N/A' }}</span>
                    </div>
                    <div class="leg-row">
                      <span class="leg-label">Delta:</span>
                      <span class="leg-value">{{ strikePreview.put_side?.long_leg?.delta?.toFixed(3) || 'N/A' }}</span>
                    </div>
                    <div class="leg-row">
                      <span class="leg-label">Bid/Ask:</span>
                      <span class="leg-value">{{ formatPrice(strikePreview.put_side?.long_leg?.bid) }}/{{ formatPrice(strikePreview.put_side?.long_leg?.ask) }}</span>
                    </div>
                    <div class="leg-row">
                      <span class="leg-label">Mid:</span>
                      <span class="leg-value highlight">{{ formatPrice(strikePreview.put_side?.long_leg?.mid) }}</span>
                    </div>
                  </div>
                </div>
              </div>
              <div class="ic-side-credit">
                <span class="spread-label">Put Side Credit:</span>
                <span class="spread-value">{{ formatPrice(strikePreview.put_side?.mid_credit) }}</span>
              </div>

              <h4 class="ic-side-title">Call Side (Bear Call Spread)</h4>
              <div class="preview-legs">
                <div class="preview-leg short-leg">
                  <h4>Short Call (Sell)</h4>
                  <div class="leg-details">
                    <div class="leg-row">
                      <span class="leg-label">Strike:</span>
                      <span class="leg-value">{{ strikePreview.call_side?.short_leg?.strike?.toFixed(0) || 'N/A' }}</span>
                    </div>
                    <div class="leg-row">
                      <span class="leg-label">Delta:</span>
                      <span class="leg-value">{{ strikePreview.call_side?.short_leg?.delta?.toFixed(3) || 'N/A' }}</span>
                    </div>
                    <div class="leg-row">
                      <span class="leg-label">Bid/Ask:</span>
                      <span class="leg-value">{{ formatPrice(strikePreview.call_side?.short_leg?.bid) }}/{{ formatPrice(strikePreview.call_side?.short_leg?.ask) }}</span>
                    </div>
                    <div class="leg-row">
                      <span class="leg-label">Mid:</span>
                      <span class="leg-value highlight">{{ formatPrice(strikePreview.call_side?.short_leg?.mid) }}</span>
                    </div>
                  </div>
                </div>
                <div class="preview-leg long-leg">
                  <h4>Long Call (Buy)</h4>
                  <div class="leg-details">
                    <div class="leg-row">
                      <span class="leg-label">Strike:</span>
                      <span class="leg-value">{{ strikePreview.call_side?.long_leg?.strike?.toFixed(0) || 'N/A' }}</span>
                    </div>
                    <div class="leg-row">
                      <span class="leg-label">Delta:</span>
                      <span class="leg-value">{{ strikePreview.call_side?.long_leg?.delta?.toFixed(3) || 'N/A' }}</span>
                    </div>
                    <div class="leg-row">
                      <span class="leg-label">Bid/Ask:</span>
                      <span class="leg-value">{{ formatPrice(strikePreview.call_side?.long_leg?.bid) }}/{{ formatPrice(strikePreview.call_side?.long_leg?.ask) }}</span>
                    </div>
                    <div class="leg-row">
                      <span class="leg-label">Mid:</span>
                      <span class="leg-value highlight">{{ formatPrice(strikePreview.call_side?.long_leg?.mid) }}</span>
                    </div>
                  </div>
                </div>
              </div>
              <div class="ic-side-credit">
                <span class="spread-label">Call Side Credit:</span>
                <span class="spread-value">{{ formatPrice(strikePreview.call_side?.mid_credit) }}</span>
              </div>

              <div class="preview-spread">
                <h4>Iron Condor Summary</h4>
                <div class="spread-details">
                  <div class="spread-row">
                    <span class="spread-label">Total Natural:</span>
                    <span class="spread-value">{{ formatPrice(strikePreview.spread?.total_natural_credit) }}</span>
                  </div>
                  <div class="spread-row">
                    <span class="spread-label">Total Mid Credit:</span>
                    <span class="spread-value highlight">{{ formatPrice(strikePreview.spread?.total_mid_credit) }}</span>
                  </div>
                  <div class="spread-row" v-if="config.trade_config.starting_offset > 0">
                    <span class="spread-label">Start Price:</span>
                    <span class="spread-value highlight-green">{{ formatPrice(calculateStartingPrice()) }}</span>
                  </div>
                </div>
              </div>
            </template>

            <!-- Credit Spread: show single side -->
            <template v-else>
              <div class="preview-legs">
                <div class="preview-leg short-leg">
                  <h4>Short (Sell)</h4>
                  <div class="leg-details">
                    <div class="leg-row">
                      <span class="leg-label">Strike:</span>
                      <span class="leg-value">{{ strikePreview.short_leg?.strike?.toFixed(0) || 'N/A' }}</span>
                    </div>
                    <div class="leg-row">
                      <span class="leg-label">Delta:</span>
                      <span class="leg-value">{{ strikePreview.short_leg?.delta?.toFixed(3) || 'N/A' }}</span>
                    </div>
                    <div class="leg-row">
                      <span class="leg-label">Bid/Ask:</span>
                      <span class="leg-value">{{ formatPrice(strikePreview.short_leg?.bid) }}/{{ formatPrice(strikePreview.short_leg?.ask) }}</span>
                    </div>
                    <div class="leg-row">
                      <span class="leg-label">Mid:</span>
                      <span class="leg-value highlight">{{ formatPrice(strikePreview.short_leg?.mid) }}</span>
                    </div>
                  </div>
                </div>
                
                <div class="preview-leg long-leg">
                  <h4>Long (Buy)</h4>
                  <div class="leg-details">
                    <div class="leg-row">
                      <span class="leg-label">Strike:</span>
                      <span class="leg-value">{{ strikePreview.long_leg?.strike?.toFixed(0) || 'N/A' }}</span>
                    </div>
                    <div class="leg-row">
                      <span class="leg-label">Delta:</span>
                      <span class="leg-value">{{ strikePreview.long_leg?.delta?.toFixed(3) || 'N/A' }}</span>
                    </div>
                    <div class="leg-row">
                      <span class="leg-label">Bid/Ask:</span>
                      <span class="leg-value">{{ formatPrice(strikePreview.long_leg?.bid) }}/{{ formatPrice(strikePreview.long_leg?.ask) }}</span>
                    </div>
                    <div class="leg-row">
                      <span class="leg-label">Mid:</span>
                      <span class="leg-value highlight">{{ formatPrice(strikePreview.long_leg?.mid) }}</span>
                    </div>
                  </div>
                </div>
              </div>
              
              <div class="preview-spread">
                <h4>Spread Summary</h4>
                <div class="spread-details">
                  <div class="spread-row">
                    <span class="spread-label">Natural:</span>
                    <span class="spread-value">{{ formatPrice(strikePreview.spread?.natural_credit) }}</span>
                  </div>
                  <div class="spread-row">
                    <span class="spread-label">Mid Credit:</span>
                    <span class="spread-value highlight">{{ formatPrice(strikePreview.spread?.mid_credit) }}</span>
                  </div>
                  <div class="spread-row" v-if="config.trade_config.starting_offset > 0">
                    <span class="spread-label">Start Price:</span>
                    <span class="spread-value highlight-green">{{ formatPrice(calculateStartingPrice()) }}</span>
                  </div>
                </div>
              </div>
            </template>
          </div>
          
          <div v-if="previewError" class="preview-error">
            {{ previewError }}
          </div>
        </div>
      </div>

      <!-- Enable/Disable Section -->
      <div class="form-section">
        <div class="enable-toggle">
          <InputSwitch v-model="config.enabled" />
          <span class="enable-label">
            {{ config.enabled ? 'Config Enabled' : 'Config Disabled' }}
          </span>
          <span class="enable-hint">
            {{ config.enabled ? 'This config will be available for automation' : 'This config will not be used for automation' }}
          </span>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { ref, computed, onMounted, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { api } from '../../services/api.js'
import { useMobileDetection } from '../../composables/useMobileDetection.js'

export default {
  name: 'AutomationConfigForm',
  setup() {
    const router = useRouter()
    const route = useRoute()
    const { isMobile } = useMobileDetection()

    // Check if we're in edit mode
    const isEditMode = computed(() => !!route.params.id)
    const configId = computed(() => route.params.id)

    // State
    const isLoading = ref(false)
    const isSaving = ref(false)
    const errors = ref({})
    
    // Indicator testing
    const testingIndicator = ref(null)
    const testingAll = ref(false)
    const indicatorResults = ref({})
    const allIndicatorsResult = ref(null)
    
    // Add indicator dialog
    const showAddIndicatorDialog = ref(false)

    // Strike preview
    const loadingPreview = ref(false)
    const strikePreview = ref(null)
    const previewError = ref(null)

    // Config data - starts with empty indicators for new configs
    const config = ref({
      name: '',
      symbol: 'NDX',
      entry_time: '12:25',
      entry_timezone: 'America/New_York',
      recurrence: 'once',
      enabled: true,
      indicators: [], // Empty - user adds indicators via Add Indicator dialog
      trade_config: {
        strategy: 'put_spread',
        width: 20,
        target_delta: 0.05,
        max_capital: 5000,
        order_type: 'limit',
        time_in_force: 'day',
        price_ladder_step: 0.05,
        max_attempts: 10,
        attempt_interval: 30,
        delta_drift_limit: 0.01,
        starting_offset: 0.10,
        min_credit: 0.30,
        expiration_mode: '0dte',
        custom_expiration: '',
        // Iron Condor per-side configs
        put_side_config: { target_delta: 0.05, width: 20 },
        call_side_config: { target_delta: 0.05, width: 20 },
      }
    })

    // Computed: is the current strategy an Iron Condor
    const isIronCondor = computed(() => config.value.trade_config.strategy === 'iron_condor')

    // Options
    const symbols = ['NDX', 'SPX']
    const timezones = ['America/New_York', 'America/Chicago', 'America/Los_Angeles', 'UTC']
    const operators = [
      { label: '> (greater than)', value: 'gt' },
      { label: '< (less than)', value: 'lt' },
      { label: '= (equals)', value: 'eq' },
      { label: '!= (not equals)', value: 'ne' },
    ]
    const mobileOperators = [
      { label: '>', value: 'gt' },
      { label: '<', value: 'lt' },
      { label: '=', value: 'eq' },
      { label: '!=', value: 'ne' },
    ]
    // Use symbols-only operators on mobile
    const displayOperators = computed(() => isMobile.value ? mobileOperators : operators)
    const strategies = [
      { label: 'Put Credit Spread', value: 'put_spread' },
      { label: 'Call Credit Spread', value: 'call_spread' },
      { label: 'Iron Condor', value: 'iron_condor' },
    ]
    const orderTypes = [
      { label: 'Limit', value: 'limit' },
      { label: 'Market', value: 'market' },
    ]
    const timeInForceOptions = [
      { label: 'Day', value: 'day' },
      { label: 'GTC (Good Till Cancel)', value: 'gtc' },
    ]
    const expirationModes = [
      { label: '0DTE (Same Day)', value: '0dte' },
      { label: '1DTE (Next Day)', value: '1dte' },
      { label: '2DTE (Two Days Out)', value: '2dte' },
      { label: 'Custom Date', value: 'custom' },
    ]
    const recurrenceOptions = [
      { label: 'Run Once', value: 'once' },
      { label: 'Daily (Repeat Each Day)', value: 'daily' },
    ]
    
    // Indicator metadata from API
    const indicatorMetadata = ref([])
    const indicatorMetadataMap = ref({})

    const fetchIndicatorMetadata = async () => {
      try {
        const response = await api.getIndicatorMetadata()
        const indicators = response.data?.indicators || []
        indicatorMetadata.value = indicators
        const map = {}
        indicators.forEach(meta => { map[meta.type] = meta })
        indicatorMetadataMap.value = map
      } catch (err) {
        console.error('Failed to fetch indicator metadata:', err)
        // Fallback: indicatorTypes computed will use hardcoded values
      }
    }

    // Indicator types for add indicator dialog (metadata-driven with fallback)
    const indicatorTypes = computed(() => {
      if (indicatorMetadata.value.length === 0) {
        // Fallback while metadata loads or if API fails
        return [
          { value: 'vix', label: 'VIX Level', description: 'CBOE Volatility Index level', category: 'market' },
          { value: 'gap', label: 'Gap %', description: '(Open - Prev Close) / Prev Close * 100', category: 'market' },
          { value: 'range', label: 'Range %', description: '(High - Low) / Open * 100', category: 'market' },
          { value: 'trend', label: 'Trend %', description: '(Current - Open) / Open * 100', category: 'market' },
          { value: 'calendar', label: 'FOMC Calendar', description: '0 = not FOMC day, 1 = FOMC day', category: 'calendar' },
        ]
      }
      return indicatorMetadata.value.map(meta => ({
        value: meta.type,
        label: meta.label,
        description: meta.description,
        category: meta.category,
      }))
    })

    const groupedIndicatorTypes = computed(() => {
      const groups = {}
      indicatorTypes.value.forEach(type => {
        const cat = type.category || 'other'
        if (!groups[cat]) groups[cat] = []
        groups[cat].push(type)
      })
      return groups
    })

    const categoryLabels = {
      market: 'Market',
      calendar: 'Calendar',
      momentum: 'Momentum',
      trend: 'Trend',
      volatility: 'Volatility',
    }

    // Methods
    const formatIndicatorType = (type) => {
      return indicatorMetadataMap.value[type]?.label || type
    }

    const getIndicatorDescription = (type) => {
      return indicatorMetadataMap.value[type]?.description || ''
    }

    const getIndicatorDefaultSymbol = (type) => {
      if (type === 'vix') return 'VIX'
      if (type === 'calendar') return ''
      return 'QQQ'
    }

    const getIndicatorParams = (type) => {
      return indicatorMetadataMap.value[type]?.params || []
    }

    const getIndicatorNeedsSymbol = (type) => {
      const meta = indicatorMetadataMap.value[type]
      if (!meta) return type !== 'calendar'
      return meta.needs_symbol
    }

    const formatIndicatorTypeWithParams = (indicator) => {
      const label = formatIndicatorType(indicator.type)
      const params = getIndicatorParams(indicator.type)
      if (params.length === 0 || !indicator.params) return label

      const values = params.map(p => {
        const val = indicator.params[p.key] ?? p.default_value
        return p.type === 'float' ? val.toFixed(1) : Math.round(val)
      })
      return `${label} (${values.join('/')})`
    }

    const getRecurrenceHint = () => {
      if (config.value.recurrence === 'daily') {
        return 'Will automatically reset and trade again each trading day'
      }
      return 'Will stop after one successful trade'
    }

    // Generate unique ID for indicators (frontend-side until saved)
    const generateIndicatorId = () => {
      return `ind_${Date.now()}_${Math.random().toString(36).substr(2, 4)}`
    }

    // Add a new indicator
    const addIndicator = (type) => {
      const meta = indicatorMetadataMap.value[type]
      const params = {}
      if (meta && meta.params && meta.params.length > 0) {
        meta.params.forEach(p => {
          params[p.key] = p.default_value
        })
      }

      const newIndicator = {
        id: generateIndicatorId(),
        type: type,
        enabled: true,
        operator: 'eq', // Default operator
        threshold: 0,   // No default value - user must set
        symbol: '',     // No default symbol - user must set
        params: Object.keys(params).length > 0 ? params : undefined,
      }
      config.value.indicators.push(newIndicator)
      showAddIndicatorDialog.value = false
    }

    // Remove an indicator
    const removeIndicator = (indicatorId) => {
      config.value.indicators = config.value.indicators.filter(ind => ind.id !== indicatorId)
      // Also remove any test results for this indicator
      delete indicatorResults.value[indicatorId]
    }

    const getIndicatorResultClass = (result) => {
      if (result.stale) return 'stale'
      return result.passed ? 'passed' : 'failed'
    }

    const loadConfig = async () => {
      if (!configId.value) return
      
      isLoading.value = true
      try {
        const response = await api.getAutomationConfig(configId.value)
        // Handle nested response format: response.data.config or response.config
        const configData = response.data?.config || response.config
        if (configData) {
          // Merge with defaults to ensure all fields exist
          config.value = {
            ...config.value,
            ...configData,
            trade_config: {
              ...config.value.trade_config,
              ...configData.trade_config
            }
          }
        }
      } catch (err) {
        console.error('Failed to load config:', err)
        alert('Failed to load config: ' + (err.response?.data?.message || err.message))
      } finally {
        isLoading.value = false
      }
    }

    const validateConfig = () => {
      errors.value = {}
      
      if (!config.value.name?.trim()) {
        errors.value.name = 'Config name is required'
      }
      
      if (!config.value.symbol) {
        errors.value.symbol = 'Symbol is required'
      }
      
      if (!config.value.entry_time?.match(/^\d{1,2}:\d{2}$/)) {
        errors.value.entry_time = 'Invalid time format (use HH:MM)'
      }
      
      return Object.keys(errors.value).length === 0
    }

    const saveConfig = async () => {
      if (!validateConfig()) return
      
      isSaving.value = true
      try {
        // Clean up indicator symbols (remove empty ones for calendar)
        const cleanedConfig = {
          ...config.value,
          indicators: config.value.indicators.map(ind => ({
            ...ind,
            symbol: ind.symbol?.trim() || undefined
          }))
        }
        
        if (isEditMode.value) {
          await api.updateAutomationConfig(configId.value, cleanedConfig)
        } else {
          await api.createAutomationConfig(cleanedConfig)
        }
        router.push('/automation')
      } catch (err) {
        console.error('Failed to save config:', err)
        alert('Failed to save config: ' + (err.response?.data?.message || err.message))
      } finally {
        isSaving.value = false
      }
    }

    const cancel = () => {
      router.push('/automation')
    }

    const testIndicator = async (indicator) => {
      testingIndicator.value = indicator.id
      try {
        const response = await api.previewAutomationIndicators({
          indicators: [indicator]
        })
        // Handle response format: data.indicators is an array
        const indicators = response.data?.indicators || response.indicators || []
        // Find result by type (API returns type, not id)
        const result = indicators.find(ind => ind.type === indicator.type)
        if (result) {
          // Map API format to our expected format, keyed by indicator ID
          indicatorResults.value[indicator.id] = {
            value: result.value,
            passed: result.pass,
            operator: result.operator,
            threshold: result.threshold,
            symbol: result.symbol,
            details: result.details,
            stale: result.stale || false,
            error: result.error || ''
          }
        }
      } catch (err) {
        console.error('Failed to test indicator:', err)
      } finally {
        testingIndicator.value = null
      }
    }

    const testAllIndicators = async () => {
      testingAll.value = true
      allIndicatorsResult.value = null
      indicatorResults.value = {}
      
      try {
        const enabledIndicators = config.value.indicators.filter(ind => ind.enabled)
        const response = await api.previewAutomationIndicators({
          indicators: enabledIndicators
        })
        // Handle response format: data.indicators is an array, data.all_pass is boolean
        const indicators = response.data?.indicators || response.indicators || []
        const allPass = response.data?.all_pass ?? response.all_pass ?? false
        
        // Convert array to object keyed by indicator ID
        // Match results to original indicators by type and index
        const enabledByType = {}
        enabledIndicators.forEach(ind => {
          if (!enabledByType[ind.type]) enabledByType[ind.type] = []
          enabledByType[ind.type].push(ind)
        })
        
        const typeCounters = {}
        indicators.forEach(result => {
          // Find the matching indicator by type (in order)
          const typeList = enabledByType[result.type] || []
          const typeIdx = typeCounters[result.type] || 0
          typeCounters[result.type] = typeIdx + 1
          
          const matchingIndicator = typeList[typeIdx]
          if (matchingIndicator) {
            indicatorResults.value[matchingIndicator.id] = {
              value: result.value,
              passed: result.pass,
              operator: result.operator,
              threshold: result.threshold,
              symbol: result.symbol,
              details: result.details,
              stale: result.stale || false,
              error: result.error || ''
            }
          }
        })
        allIndicatorsResult.value = allPass
      } catch (err) {
        console.error('Failed to test indicators:', err)
        allIndicatorsResult.value = false
      } finally {
        testingAll.value = false
      }
    }

    const loadStrikePreview = async () => {
      loadingPreview.value = true
      strikePreview.value = null
      previewError.value = null
      
      try {
        const tc = config.value.trade_config
        let payload = {
          symbol: config.value.symbol,
          strategy: tc.strategy,
          expiration_mode: tc.expiration_mode || '0dte',
          custom_expiration: tc.custom_expiration || '',
        }

        if (tc.strategy === 'iron_condor') {
          // Iron Condor: send per-side config
          payload.put_target_delta = tc.put_side_config.target_delta
          payload.put_width = tc.put_side_config.width
          payload.call_target_delta = tc.call_side_config.target_delta
          payload.call_width = tc.call_side_config.width
        } else {
          // Credit spread: send single delta/width
          payload.target_delta = tc.target_delta
          payload.width = tc.width
        }

        const response = await api.previewStrikes(payload)
        
        if (response.success && response.data) {
          strikePreview.value = response.data
        } else {
          previewError.value = response.message || 'Failed to load strike preview'
        }
      } catch (err) {
        console.error('Failed to load strike preview:', err)
        previewError.value = err.response?.data?.message || err.message || 'Failed to load strike preview'
      } finally {
        loadingPreview.value = false
      }
    }

    const formatPrice = (price) => {
      if (price === null || price === undefined) return 'N/A'
      return '$' + price.toFixed(2)
    }

    const calculateStartingPrice = () => {
      // For iron condor, use total_mid_credit from spread
      const midCredit = strikePreview.value?.spread?.mid_credit 
        || strikePreview.value?.spread?.total_mid_credit
      if (!midCredit) return null
      const offset = config.value.trade_config.starting_offset || 0
      return midCredit - offset
    }

    // Lifecycle
    onMounted(async () => {
      await fetchIndicatorMetadata()
      if (isEditMode.value) {
        await loadConfig()
      }
    })

    return {
      // State
      config,
      isEditMode,
      isLoading,
      isSaving,
      errors,
      testingIndicator,
      testingAll,
      indicatorResults,
      allIndicatorsResult,
      loadingPreview,
      strikePreview,
      previewError,
      isIronCondor,

      // Options
      symbols,
      timezones,
      operators,
      mobileOperators,
      displayOperators,
      strategies,
      orderTypes,
      timeInForceOptions,
      expirationModes,
      recurrenceOptions,
      indicatorTypes,
      groupedIndicatorTypes,
      categoryLabels,
      
      // Mobile
      isMobile,
      
      // Add indicator dialog
      showAddIndicatorDialog,

      // Methods
      formatIndicatorType,
      formatIndicatorTypeWithParams,
      getIndicatorDescription,
      getIndicatorDefaultSymbol,
      getIndicatorParams,
      getIndicatorNeedsSymbol,
      getRecurrenceHint,
      getIndicatorResultClass,
      addIndicator,
      removeIndicator,
      saveConfig,
      cancel,
      testIndicator,
      testAllIndicators,
      loadStrikePreview,
      formatPrice,
      calculateStartingPrice,
    }
  }
}
</script>

<style scoped>
.automation-config-form {
  padding: var(--spacing-lg);
  width: 100%;
  height: 100%;
  overflow-y: auto;
}

/* Header */
.form-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: var(--spacing-xl);
  padding-bottom: var(--spacing-lg);
  border-bottom: 1px solid var(--border-primary);
}

.header-content {
  flex: 1;
}

.form-title {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  font-size: var(--font-size-2xl);
  font-weight: var(--font-weight-bold);
  color: var(--text-primary);
  margin: 0 0 var(--spacing-xs) 0;
}

.form-title i {
  color: var(--color-brand);
}

.form-subtitle {
  color: var(--text-secondary);
  font-size: var(--font-size-md);
  margin: 0;
}

.header-actions {
  display: flex;
  gap: var(--spacing-sm);
}

/* Loading State */
.loading-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: var(--spacing-2xl);
  text-align: center;
}

.loading-spinner {
  width: 32px;
  height: 32px;
  border: 3px solid var(--border-secondary);
  border-top: 3px solid var(--color-brand);
  border-radius: 50%;
  animation: spin 1s linear infinite;
  margin-bottom: var(--spacing-md);
}

@keyframes spin {
  0% { transform: rotate(0deg); }
  100% { transform: rotate(360deg); }
}

/* Form Content */
.form-content {
  max-width: 900px;
}

.form-section {
  background: var(--bg-secondary);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-lg);
  padding: var(--spacing-lg);
  margin-bottom: var(--spacing-lg);
}

.section-title {
  font-size: var(--font-size-lg);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
  margin: 0 0 var(--spacing-md) 0;
}

.section-description {
  color: var(--text-secondary);
  font-size: var(--font-size-sm);
  margin: 0 0 var(--spacing-md) 0;
}

.subsection-title {
  font-size: var(--font-size-md);
  font-weight: var(--font-weight-medium);
  color: var(--text-primary);
  margin: var(--spacing-lg) 0 var(--spacing-md) 0;
  padding-top: var(--spacing-md);
  border-top: 1px solid var(--border-primary);
}

.form-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: var(--spacing-md);
}

.form-field {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-xs);
}

.form-field label {
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-medium);
  color: var(--text-secondary);
}

.field-hint {
  font-size: var(--font-size-xs);
  color: var(--text-tertiary);
}

.p-error {
  color: var(--color-danger);
}

/* Indicators List */
.indicators-list {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-sm);
}

.indicator-row {
  display: flex;
  align-items: center;
  gap: var(--spacing-md);
  padding: var(--spacing-md);
  background: var(--bg-tertiary);
  border-radius: var(--radius-md);
  transition: opacity 0.2s;
}

.indicator-row.disabled {
  opacity: 0.5;
}

.indicator-toggle {
  flex-shrink: 0;
}

.indicator-type {
  flex: 0 0 200px;
  min-width: 150px;
  max-width: 200px;
}

.type-label {
  display: block;
  font-weight: var(--font-weight-medium);
  color: var(--text-primary);
}

.type-description {
  display: block;
  font-size: var(--font-size-xs);
  color: var(--text-tertiary);
}

.indicator-config {
  display: flex;
  gap: 12px;
  align-items: center;
}

.indicator-config.dimmed {
  opacity: 0.5;
}

.operator-dropdown {
  flex: 0 0 auto;
}

:deep(.operator-dropdown.p-dropdown) {
  width: 220px;
}

.threshold-input {
  flex: 0 0 auto;
}

:deep(.threshold-input.p-inputnumber) {
  width: 100px;
}

:deep(.threshold-input .p-inputnumber-input) {
  width: 100px;
  border-radius: var(--radius-sm);
}

.symbol-input {
  flex: 0 0 auto;
}

:deep(.symbol-input.p-inputtext) {
  width: 80px;
  border-radius: var(--radius-sm);
}

.indicator-test {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  flex: 0 0 auto;
}

.test-result {
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-medium);
  padding: 2px 8px;
  border-radius: var(--radius-sm);
}

.test-result.passed {
  background: rgba(34, 197, 94, 0.1);
  color: var(--color-success);
}

.test-result.failed {
  background: rgba(239, 68, 68, 0.1);
  color: var(--color-danger);
}

.test-result.stale {
  background: rgba(251, 191, 36, 0.15);
  color: var(--color-warning);
  border: 1px dashed var(--color-warning);
}

.test-result .stale-icon,
.mobile-test-result .stale-icon {
  font-size: 10px;
  margin-right: 2px;
}

/* Indicators Empty State */
.indicators-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: var(--spacing-xl);
  text-align: center;
  background: var(--bg-tertiary);
  border-radius: var(--radius-md);
  border: 1px dashed var(--border-secondary);
  margin-bottom: var(--spacing-md);
}

.indicators-empty i {
  font-size: var(--font-size-2xl);
  color: var(--text-tertiary);
  margin-bottom: var(--spacing-sm);
}

.indicators-empty p {
  color: var(--text-secondary);
  margin: 0;
}

/* Indicator Remove Button */
.indicator-remove {
  flex-shrink: 0;
}

/* Indicators Actions (Add + Test All) */
.indicators-actions {
  display: flex;
  align-items: center;
  gap: var(--spacing-md);
  margin-top: var(--spacing-md);
  padding-top: var(--spacing-md);
  border-top: 1px solid var(--border-primary);
}

.test-all-section {
  display: flex;
  align-items: center;
  gap: var(--spacing-md);
  margin-top: var(--spacing-md);
  padding-top: var(--spacing-md);
  border-top: 1px solid var(--border-primary);
}

.all-result {
  font-weight: var(--font-weight-semibold);
  padding: var(--spacing-xs) var(--spacing-sm);
  border-radius: var(--radius-sm);
}

.all-result.passed {
  background: rgba(34, 197, 94, 0.1);
  color: var(--color-success);
}

.all-result.failed {
  background: rgba(239, 68, 68, 0.1);
  color: var(--color-danger);
}

/* Strike Preview Section */
.strike-preview-section {
  margin-top: var(--spacing-md);
}

.strike-preview-content {
  margin-top: var(--spacing-md);
  padding: var(--spacing-md);
  background: var(--bg-tertiary);
  border-radius: var(--radius-md);
  border: 1px solid var(--border-primary);
}

.preview-expiry {
  margin-bottom: var(--spacing-md);
  padding-bottom: var(--spacing-sm);
  border-bottom: 1px solid var(--border-primary);
  color: var(--text-secondary);
}

.preview-legs {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: var(--spacing-md);
  margin-bottom: var(--spacing-md);
}

.preview-leg {
  padding: var(--spacing-sm);
  border-radius: var(--radius-sm);
  background: var(--bg-secondary);
}

.preview-leg h4 {
  margin: 0 0 var(--spacing-sm) 0;
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
}

.preview-leg.short-leg h4 {
  color: var(--color-danger);
}

.preview-leg.long-leg h4 {
  color: var(--color-success);
}

.leg-details, .spread-details {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-xs);
}

.leg-row, .spread-row {
  display: flex;
  justify-content: space-between;
  font-size: var(--font-size-sm);
}

.leg-label, .spread-label {
  color: var(--text-tertiary);
}

.leg-value, .spread-value {
  color: var(--text-primary);
  font-family: monospace;
}

.leg-value.highlight, .spread-value.highlight {
  font-weight: var(--font-weight-semibold);
  color: var(--color-brand);
}

.spread-value.highlight-green {
  font-weight: var(--font-weight-semibold);
  color: var(--color-success);
}

.preview-spread {
  padding: var(--spacing-sm);
  border-radius: var(--radius-sm);
  background: var(--bg-secondary);
  border: 1px solid var(--color-brand);
}

.preview-spread h4 {
  margin: 0 0 var(--spacing-sm) 0;
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
}

.preview-error {
  margin-top: var(--spacing-md);
  padding: var(--spacing-sm);
  background: rgba(239, 68, 68, 0.1);
  border: 1px solid var(--color-danger);
  border-radius: var(--radius-sm);
  color: var(--color-danger);
  font-size: var(--font-size-sm);
}

/* Iron Condor Side Headers */
.ic-side-title {
  margin: var(--spacing-md) 0 var(--spacing-sm) 0;
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-semibold);
  color: var(--text-secondary);
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.ic-side-credit {
  display: flex;
  justify-content: space-between;
  padding: var(--spacing-xs) var(--spacing-sm);
  margin-bottom: var(--spacing-sm);
  font-size: var(--font-size-sm);
  color: var(--text-secondary);
  border-bottom: 1px solid var(--border-primary);
}

/* Enable Toggle */
.enable-toggle {
  display: flex;
  align-items: center;
  gap: var(--spacing-md);
}

.enable-label {
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
}

.enable-hint {
  font-size: var(--font-size-sm);
  color: var(--text-tertiary);
}

/* Responsive */
@media (max-width: 768px) {
  .automation-config-form {
    padding: var(--spacing-md);
  }

  .form-header {
    flex-direction: column;
    gap: var(--spacing-md);
  }

  .form-grid {
    grid-template-columns: 1fr;
  }

  .indicator-row {
    flex-wrap: wrap;
  }

  .indicator-config {
    width: 100%;
    flex-wrap: wrap;
  }
}

/* Mobile Indicators - Single Line Layout */
.mobile-indicators .indicator-row {
  flex-wrap: nowrap;
  padding: var(--spacing-sm);
  gap: var(--spacing-xs);
  align-items: center;
}

.mobile-indicators .indicator-toggle {
  flex: 0 0 auto;
}

.mobile-indicators .indicator-type {
  flex: 0 0 auto;
  min-width: auto;
  max-width: none;
}

.mobile-indicators .type-label {
  font-size: var(--font-size-sm);
  white-space: nowrap;
}

.mobile-indicators .indicator-config {
  flex: 1;
  display: flex;
  gap: var(--spacing-xs);
  flex-wrap: nowrap;
  justify-content: flex-end;
  min-width: 0;
}

.mobile-indicators .indicator-config.dimmed {
  opacity: 0.5;
}

/* Mobile operator dropdown - compact, no trigger arrow */
:deep(.mobile-operator.p-dropdown) {
  width: 44px !important;
  min-width: 44px !important;
}

:deep(.mobile-operator .p-dropdown-label) {
  padding: 6px 8px;
  font-size: var(--font-size-sm);
  text-align: center;
  text-overflow: clip;
  overflow: visible;
}

:deep(.mobile-operator .p-dropdown-trigger) {
  display: none;
}

/* Mobile threshold input - compact */
:deep(.mobile-threshold.p-inputnumber) {
  width: 60px !important;
}

:deep(.mobile-threshold .p-inputnumber-input) {
  width: 60px !important;
  padding: 6px 4px;
  font-size: var(--font-size-sm);
  text-align: center;
}

/* Mobile symbol input - compact */
:deep(.mobile-symbol.p-inputtext) {
  width: 50px !important;
  padding: 6px 4px;
  font-size: var(--font-size-sm);
  text-align: center;
}

/* Mobile test result - inline display */
.mobile-test-result {
  font-size: var(--font-size-xs);
  font-weight: var(--font-weight-semibold);
  padding: 4px 6px;
  border-radius: var(--radius-sm);
  white-space: nowrap;
  min-width: 40px;
  text-align: center;
}

.mobile-test-result.passed {
  background: rgba(34, 197, 94, 0.15);
  color: var(--color-success);
}

.mobile-test-result.failed {
  background: rgba(239, 68, 68, 0.15);
  color: var(--color-danger);
}

.mobile-test-result.stale {
  background: rgba(251, 191, 36, 0.15);
  color: var(--color-warning);
  border: 1px dashed var(--color-warning);
}

/* Mobile Strike Preview */
.mobile-preview {
  padding: var(--spacing-sm);
}

.mobile-preview .preview-expiry {
  font-size: var(--font-size-sm);
  margin-bottom: var(--spacing-sm);
  padding-bottom: var(--spacing-xs);
}

.mobile-preview .preview-legs {
  grid-template-columns: 1fr 1fr;
  gap: var(--spacing-xs);
  margin-bottom: var(--spacing-sm);
}

.mobile-preview .preview-leg {
  padding: var(--spacing-xs);
}

.mobile-preview .preview-leg h4 {
  font-size: var(--font-size-xs);
  margin-bottom: var(--spacing-xs);
}

.mobile-preview .leg-details {
  gap: 2px;
}

.mobile-preview .leg-row {
  font-size: var(--font-size-xs);
}

.mobile-preview .leg-label {
  font-size: var(--font-size-xs);
}

.mobile-preview .leg-value {
  font-size: var(--font-size-xs);
}

.mobile-preview .preview-spread {
  padding: var(--spacing-xs);
}

.mobile-preview .preview-spread h4 {
  font-size: var(--font-size-xs);
  margin-bottom: var(--spacing-xs);
}

.mobile-preview .spread-details {
  gap: 2px;
}

.mobile-preview .spread-row {
  font-size: var(--font-size-xs);
}

.mobile-preview .spread-label {
  font-size: var(--font-size-xs);
}

.mobile-preview .spread-value {
  font-size: var(--font-size-xs);
}

/* Add Indicator Dialog */
.add-indicator-content {
  padding: var(--spacing-sm);
}

.add-indicator-description {
  color: var(--text-secondary);
  margin: 0 0 var(--spacing-md) 0;
  font-size: var(--font-size-sm);
}

.indicator-type-options {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-sm);
}

.indicator-type-option {
  padding: var(--spacing-md);
  background: var(--bg-tertiary);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-md);
  cursor: pointer;
  transition: var(--transition-fast);
}

.indicator-type-option:hover {
  border-color: var(--color-brand);
  background: var(--bg-secondary);
}

.indicator-type-option .option-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: var(--spacing-xs);
}

.indicator-type-option .option-label {
  font-weight: var(--font-weight-semibold);
  color: var(--text-primary);
}

.indicator-type-option .option-description {
  font-size: var(--font-size-sm);
  color: var(--text-secondary);
  margin: 0;
}

/* Parameter inputs for technical indicators */
.param-input-group {
  display: flex;
  align-items: center;
  gap: 4px;
}

.param-label {
  font-size: 0.75rem;
  color: var(--text-tertiary, var(--text-secondary));
  white-space: nowrap;
}

:deep(.param-value-input.p-inputnumber) {
  width: 70px;
}

:deep(.param-value-input .p-inputnumber-input) {
  width: 70px;
  border-radius: var(--radius-sm);
}

/* Category headers in Add Indicator dialog */
.category-header {
  color: var(--color-brand);
  font-size: 0.85rem;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  margin: 12px 0 6px 0;
  padding-bottom: 4px;
  border-bottom: 1px solid var(--border-primary);
}

.category-header:first-child {
  margin-top: 0;
}

@media (max-width: 768px) {
  .param-input-group {
    flex-direction: column;
    align-items: flex-start;
  }
  :deep(.param-value-input.p-inputnumber) {
    width: 100%;
  }
}
</style>
