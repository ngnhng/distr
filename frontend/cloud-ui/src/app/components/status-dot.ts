import {NgClass} from '@angular/common';
import {Component, Input} from '@angular/core';
import {isStale} from '../../util/model';
import {DeploymentTarget} from '../types/deployment-target';

@Component({
  selector: 'app-status-dot',
  template: '<div [ngClass]="classList"></div>',
  imports: [NgClass],
})
export class StatusDotComponent {
  @Input({required: true})
  public deploymentTarget!: DeploymentTarget;

  protected get classList(): string[] {
    return ['rounded-full', 'w-full', 'h-full', this.bgClass];
  }

  private get bgClass(): string {
    const dt = this.deploymentTarget;
    if (dt !== undefined) {
      if (dt.deployment !== undefined) {
        if (dt.deployment.latestStatus !== undefined) {
          if (dt.deployment.latestStatus.type === 'ok') {
            if (isStale(dt.deployment.latestStatus)) {
              return 'bg-yellow-300';
            } else {
              return 'bg-lime-600';
            }
          } else {
            return 'bg-red-400';
          }
        }
      }
      if (dt.currentStatus !== undefined) {
        if (isStale(dt.currentStatus)) {
          return 'bg-yellow-300';
        } else {
          return 'bg-lime-600';
        }
      }
    }
    return 'bg-gray-500';
  }
}
