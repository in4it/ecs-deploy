import { Component, OnInit, ViewChild } from '@angular/core';
import { NgbModal, ModalDismissReasons } from '@ng-bootstrap/ng-bootstrap';
import { ServiceDetail, ServiceDetailService }  from './service-detail.service';

import * as moment from 'moment';

@Component({
  selector: 'app-service-detail-inspect',
  templateUrl: './inspect.component.html',
})
export class InspectChildComponent implements OnInit {

  closeResult: string;
  selectedVersion: string;
  loading: boolean = false;
  serviceName: string;
  deployment: any

  @ViewChild('inspect', { static: true }) inspectModal : NgbModal;

  constructor(
    private modalService: NgbModal,
    private sds: ServiceDetailService
  ) {}

  ngOnInit(): void { 

  }

  open(serviceName, selectedVersion) {
    if(!selectedVersion) {
      return
    }
    this.loading = true
    this.serviceName = serviceName
    this.selectedVersion = selectedVersion
    this.modalService.open(this.inspectModal, { windowClass: 'inspect-modal' } ).result.then((result) => {
      this.closeResult = `Closed with: ${result}`;
    }, (reason) => {
      this.closeResult = `Dismissed ${this.getDismissReason(reason)}`;
    });
    this.sds.getDeployment(selectedVersion).subscribe(data => {
      this.loading = false
      data['deployment']["deploymentMoment"] = moment(this.selectedVersion).format('LLL') + " UTC"
      this.deployment = data["deployment"]
    })
  }
  private getDismissReason(reason: any): string {
    if (reason === ModalDismissReasons.ESC) {
      return 'by pressing ESC';
    } else if (reason === ModalDismissReasons.BACKDROP_CLICK) {
      return 'by clicking on a backdrop';
    } else {
      return  `with: ${reason}`;
    }
  }
}
